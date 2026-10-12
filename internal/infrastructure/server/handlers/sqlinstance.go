package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	machineService "github.com/su3i/wimp/internal/application/machine"
	projectService "github.com/su3i/wimp/internal/application/project"
	sqlInstanceService "github.com/su3i/wimp/internal/application/sqlinstance"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

// RetrieveSqlInstances lists a machine's SQL Server instances. Unpaginated: a host carries a
// handful of instances at most, unlike app pools.
func RetrieveSqlInstances(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	machineID, err := strconv.ParseUint(c.Param("machineId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
		return
	}

	if err := machineService.BelongsToProject(uint(machineID), c.Param("key"), config.Database()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	instances, err := sqlInstanceService.ListByMachine(uint(machineID), config.Database())
	if err != nil {
		log.Printf("Error retrieving sql instances: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "sql_instances": instances})
}

// QuerySqlInstance runs one live, point-in-time query (blocking chain, running requests,
// Agent jobs) on an instance and returns its rows.
func QuerySqlInstance(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	kind := c.Param("kind")
	switch kind {
	case protocol.SqlQueryBlocking, protocol.SqlQueryRequests, protocol.SqlQueryJobs:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown query kind"})
		return
	}

	m, ok := machineInProject(c)
	if !ok {
		return
	}

	instance := c.Param("name")
	target, err := sqlInstanceService.QueryTarget(*m, instance, config.Database())
	if errors.Is(err, sqlInstanceService.ErrInstanceNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if !hub.Get().IsOnline(target) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "the host running this instance is not connected"})
		return
	}

	reqID := uuid.New().String()
	ch := hub.RegisterCommand(reqID)
	defer hub.DeregisterCommand(reqID)

	payload, _ := json.Marshal(protocol.SqlQueryPayload{RequestID: reqID, Instance: instance, Kind: kind})
	msg, _ := json.Marshal(protocol.Message{Type: protocol.TypeSqlQuery, Payload: payload})
	if err := hub.Get().Send(target, msg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send query"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), commandTimeout)
	defer cancel()

	select {
	case result := <-ch:
		if !result.Success {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": result.Error})
			return
		}
		c.Data(http.StatusOK, "application/json", []byte(`{"message":"success","rows":`+result.Output+`}`))
	case <-ctx.Done():
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "query timed out"})
	}
}

// sqlCommandTimeout is longer than other commands: a SQL Server restart waits on recovery
// of every database, and a cluster move on the role coming online on the new node.
const sqlCommandTimeout = 3 * time.Minute

// SqlInstanceCommand starts, restarts or moves (FCI only) a SQL Server instance. Stopping
// is deliberately not offered, the same as remote host shutdown.
func SqlInstanceCommand(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
		if err != nil || !allow {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		m, ok := machineInProject(c)
		if !ok {
			return
		}
		inst, err := sqlInstanceService.FindOnMachine(m.ID, c.Param("name"), config.Database())
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		args := map[string]string{}
		if action == protocol.SqlActionMove {
			var body struct {
				Node string `json:"node"`
			}
			_ = c.ShouldBindJSON(&body)
			if !inst.Clustered {
				c.JSON(http.StatusBadRequest, gin.H{"error": "only a clustered instance can be moved"})
				return
			}
			if err := validateTargetNode(m, body.Node, inst.OwnerNode); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			args["node"] = body.Node
		} else {
			// The stop and start this causes are expected, not an outage.
			sqlInstanceService.MarkPlanned(m.ProjectID, *m, *inst)
		}

		// Cluster cmdlets act on the whole cluster from any node, so a clustered instance is
		// commanded through the node it was looked up on, like a standalone one.
		dispatchCommand(c, m.ID, protocol.CommandPayload{
			Action: action, TargetType: protocol.TargetSqlInstance, Target: inst.Name, Args: args,
		})
	}
}

// FailoverAvailabilityGroup performs a planned failover of an availability group to the
// replica on the given node. The agent on that node refuses unless its replica is a
// synchronous-commit secondary with every database SYNCHRONIZED.
func FailoverAvailabilityGroup(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	m, ok := machineInProject(c)
	if !ok {
		return
	}
	var body struct {
		Node string `json:"node"`
	}
	_ = c.ShouldBindJSON(&body)

	agName := c.Param("ag")
	var primary string
	found := false
	if m.Cluster != nil {
		for _, ag := range m.Cluster.AvailabilityGroups {
			if ag.Name == agName {
				primary, found = ag.PrimaryNode, true
			}
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "availability group not found on this host's cluster"})
		return
	}
	if err := validateTargetNode(m, body.Node, primary); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// A planned AG failover has to be issued on the replica becoming primary.
	target, err := sqlInstanceService.MachineByHostname(m.ProjectID, body.Node, config.Database())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	dispatchCommand(c, target, protocol.CommandPayload{
		Action: protocol.SqlActionFailover, TargetType: protocol.TargetAvailabilityGroup, Target: agName,
	})
}

func validateTargetNode(m *machineDomain.Machine, node, current string) error {
	if node == "" {
		return errors.New("choose a target node")
	}
	if strings.EqualFold(node, current) {
		return errors.New(node + " is already running it")
	}
	if m.Cluster != nil {
		for _, n := range m.Cluster.Nodes {
			if strings.EqualFold(n.Name, node) {
				if n.State != "Up" {
					return errors.New(node + " is " + strings.ToLower(n.State) + ", not up")
				}
				return nil
			}
		}
	}
	return errors.New(node + " is not a node of this cluster")
}

func machineInProject(c *gin.Context) (*machineDomain.Machine, bool) {
	machineID, err := strconv.ParseUint(c.Param("machineId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
		return nil, false
	}
	if err := machineService.BelongsToProject(uint(machineID), c.Param("key"), config.Database()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return nil, false
	}
	m, err := database.NewMachineRepository(config.Database()).FindOneByID(uint(machineID))
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return nil, false
	}
	return m, true
}

// dispatchCommand sends a command to an agent and writes its result as the response.
func dispatchCommand(c *gin.Context, machineID uint, cmd protocol.CommandPayload) {
	if !hub.Get().IsOnline(machineID) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "the host is not connected"})
		return
	}

	cmd.CommandID = uuid.New().String()
	ch := hub.RegisterCommand(cmd.CommandID)
	defer hub.DeregisterCommand(cmd.CommandID)

	payload, _ := json.Marshal(cmd)
	msg, _ := json.Marshal(protocol.Message{Type: protocol.TypeCommand, Payload: payload})
	if err := hub.Get().Send(machineID, msg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send command"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), sqlCommandTimeout)
	defer cancel()

	select {
	case result := <-ch:
		if result.Success {
			c.JSON(http.StatusOK, gin.H{"message": "success", "output": result.Output})
		} else {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": result.Error, "output": result.Output})
		}
	case <-ctx.Done():
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "command is still running on the host; check its state shortly"})
	}
}

// RetrieveProjectSqlInstances lists every SQL instance row in a project, for choosing
// application members. A clustered instance appears once per node.
func RetrieveProjectSqlInstances(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	proj, err := projectService.RetrieveProject(c.Param("key"), config.Database())
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	instances, err := sqlInstanceService.ListByProject(proj.ID, config.Database())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "sql_instances": instances})
}
