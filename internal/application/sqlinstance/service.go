// Package sqlinstance keeps discovered SQL Server instances and the failover cluster they
// may belong to in sync, and turns state changes into alerts.
package sqlinstance

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	notificationService "github.com/su3i/wimp/internal/application/notification"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/domain/sqlinstance"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

// Alert is one alert produced by Evaluate, for the caller to emit.
type Alert struct {
	Type      notification.AlertType
	MachineID uint
	Instance  string
	Subject   string
	Title     string
	Detail    string
}

// Alert state is held in memory and keyed by scope rather than by reporting machine. Every
// node of a cluster reports the same cluster-wide view, so the first node to report a change
// produces the alert and the rest find it already recorded. The first observation of a key
// only sets a baseline, so a control plane restart re-learns state silently instead of
// re-alerting everything that is already down.
//
// ponytail: in-process state, same single-replica assumption as the metrics checker and the
// incident mutex. Several control plane replicas would need this moved into the database.
var (
	mu      sync.Mutex
	down    = map[string]bool{}
	owners  = map[string]string{}
	planned = map[string]time.Time{}
	now     = time.Now
)

// plannedWindow is how long after an operator starts or restarts an instance its up/down
// transitions are expected rather than alerted. Edge state is left untouched for the window,
// so when it ends the instance is compared with what was last announced: a restart that
// worked is silent, one that left the instance down alerts then.
const plannedWindow = 5 * time.Minute

// scope identifies an instance for alerting. A clustered instance belongs to the cluster,
// not to whichever node reports it: its incident has to open on one node and close on
// another after a failover.
func scope(projectID, machineID uint, hostname, name string, clustered bool, virtualServer string) (scopeMachine uint, subject, display, key string) {
	scopeMachine, subject, display = machineID, name, hostname
	if clustered && virtualServer != "" {
		scopeMachine = 0
		subject = "fci:" + virtualServer + `\` + name
		display = virtualServer
	}
	return scopeMachine, subject, display, fmt.Sprintf("%d:%d:%s", projectID, scopeMachine, subject)
}

// MarkPlanned records that an operator is about to start or restart an instance.
func MarkPlanned(projectID uint, m machine.Machine, inst sqlinstance.SqlInstance) {
	_, _, _, key := scope(projectID, m.ID, m.Hostname, inst.Name, inst.Clustered, inst.VirtualServerName)
	mu.Lock()
	planned[key] = now().Add(plannedWindow)
	mu.Unlock()
}

func Sync(machineID uint, instances []protocol.SqlInstanceInfo, cfg *config.DatabaseConfig) error {
	return database.NewSqlInstanceRepository(cfg).Sync(machineID, toRecords(instances))
}

func UpdateStates(machineID uint, instances []protocol.SqlInstanceInfo, cfg *config.DatabaseConfig) error {
	if len(instances) == 0 {
		return nil
	}
	return database.NewSqlInstanceRepository(cfg).UpdateStates(machineID, toRecords(instances))
}

func ListByMachine(machineID uint, cfg *config.DatabaseConfig) (*[]sqlinstance.SqlInstance, error) {
	return database.NewSqlInstanceRepository(cfg).FindByMachineID(machineID)
}

// QueryTarget resolves which machine can answer a live query for an instance seen on
// machineID. A clustered instance runs on whichever node owns it, which can be any node in
// the project, so the query goes there rather than to the node it was looked up on.
func QueryTarget(m machine.Machine, instanceName string, cfg *config.DatabaseConfig) (uint, error) {
	instances, err := ListByMachine(m.ID, cfg)
	if err != nil {
		return 0, err
	}
	for _, inst := range *instances {
		if inst.Name != instanceName {
			continue
		}
		if !inst.Clustered || inst.OwnerNode == "" || strings.EqualFold(inst.OwnerNode, m.Hostname) {
			return m.ID, nil
		}
		return MachineByHostname(m.ProjectID, inst.OwnerNode, cfg)
	}
	return 0, ErrInstanceNotFound
}

var ErrInstanceNotFound = errors.New("sql instance not found")

func FindOnMachine(machineID uint, name string, cfg *config.DatabaseConfig) (*sqlinstance.SqlInstance, error) {
	instances, err := ListByMachine(machineID, cfg)
	if err != nil {
		return nil, err
	}
	for _, inst := range *instances {
		if inst.Name == name {
			return &inst, nil
		}
	}
	return nil, ErrInstanceNotFound
}

// MachineByHostname finds the live machine row for a cluster node within a project.
func MachineByHostname(projectID uint, hostname string, cfg *config.DatabaseConfig) (uint, error) {
	peers, err := database.NewMachineRepository(cfg).FindByProjectID(projectID)
	if err != nil {
		return 0, err
	}
	for _, p := range *peers {
		if !p.Status.Terminal() && strings.EqualFold(p.Hostname, hostname) {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("%s is not a host in this project", hostname)
}

// InstanceWithMachine is an instance row together with the host it was reported from.
type InstanceWithMachine struct {
	sqlinstance.SqlInstance
	Machine machine.Machine `json:"machine"`
}

// ListByProject returns every instance row in a project. A clustered instance appears once
// per node it is installed on.
func ListByProject(projectID uint, cfg *config.DatabaseConfig) ([]InstanceWithMachine, error) {
	machines, err := database.NewMachineRepository(cfg).FindByProjectID(projectID)
	if err != nil {
		return nil, err
	}
	repo := database.NewSqlInstanceRepository(cfg)
	out := []InstanceWithMachine{}
	for _, m := range *machines {
		if m.Status.Terminal() {
			continue
		}
		instances, err := repo.FindByMachineID(m.ID)
		if err != nil {
			return nil, err
		}
		for _, inst := range *instances {
			out = append(out, InstanceWithMachine{SqlInstance: inst, Machine: m})
		}
	}
	return out, nil
}

// Current returns the freshest view of an instance: for a clustered one, the row of a node
// that is online, since a row from an offline node is stale.
func Current(row InstanceWithMachine, all []InstanceWithMachine) InstanceWithMachine {
	if !row.Clustered || row.Machine.Status == machine.Online {
		return row
	}
	for _, other := range all {
		if other.Clustered && other.Machine.Status == machine.Online &&
			other.VirtualServerName == row.VirtualServerName && other.Name == row.Name {
			other.ID = row.ID // keep the member identity the application refers to
			return other
		}
	}
	return row
}

func toRecords(instances []protocol.SqlInstanceInfo) []sqlinstance.SqlInstance {
	records := make([]sqlinstance.SqlInstance, 0, len(instances))
	for _, i := range instances {
		records = append(records, sqlinstance.SqlInstance{
			Name:              i.Name,
			ServiceName:       i.ServiceName,
			AgentServiceName:  i.AgentServiceName,
			Version:           i.Version,
			Edition:           i.Edition,
			Port:              i.Port,
			State:             i.State,
			AgentState:        i.AgentState,
			Clustered:         i.Clustered,
			VirtualServerName: i.VirtualServerName,
			OwnerNode:         i.OwnerNode,
			Monitored:         i.Monitored,
			MonitorError:      i.MonitorError,
		})
	}
	return records
}

// ToCluster converts the wire snapshot into its stored form.
func ToCluster(c *protocol.ClusterInfo) *machine.Cluster {
	if c == nil {
		return nil
	}
	out := &machine.Cluster{Name: c.Name}
	for _, n := range c.Nodes {
		out.Nodes = append(out.Nodes, machine.ClusterNode{Name: n.Name, State: n.State})
	}
	for _, ag := range c.AvailabilityGroups {
		out.AvailabilityGroups = append(out.AvailabilityGroups, machine.AvailabilityGroup{Name: ag.Name, PrimaryNode: ag.PrimaryNode, State: ag.State})
	}
	return out
}

// Evaluate folds one heartbeat's SQL and cluster snapshot into the alert state and returns
// the alerts it produced.
func Evaluate(projectID, machineID uint, hostname string, instances []protocol.SqlInstanceInfo, cluster *protocol.ClusterInfo) []Alert {
	mu.Lock()
	defer mu.Unlock()

	var alerts []Alert

	for _, inst := range instances {
		scopeMachine, subject, display, key := scope(projectID, machineID, hostname, inst.Name, inst.Clustered, inst.VirtualServerName)
		where := instanceLabel(inst.Name, display)

		if inst.Clustered {
			if from, moved := ownerChanged(key+":owner", inst.OwnerNode); moved {
				alerts = append(alerts, Alert{notification.AlertSqlInstanceFailover, scopeMachine, display, "",
					notificationService.AlertTitle(display, "SQL Instance Failover: "+inst.Name),
					fmt.Sprintf("SQL Server instance %s moved from %s to %s", where, from, inst.OwnerNode)})
			}
		}

		if until, ok := planned[key]; ok {
			if now().Before(until) {
				continue
			}
			delete(planned, key)
		}

		switch edge(key+":engine", inst.State) {
		case wentDown:
			alerts = append(alerts, Alert{notification.AlertSqlInstanceStopped, scopeMachine, display, subject,
				notificationService.AlertTitle(display, "SQL Instance "+inst.State+": "+inst.Name),
				fmt.Sprintf("SQL Server instance %s is %s", where, strings.ToLower(inst.State))})
		case cameUp:
			alerts = append(alerts, Alert{notification.AlertSqlInstanceStarted, scopeMachine, display, subject,
				notificationService.AlertTitle(display, "SQL Instance Running: "+inst.Name),
				fmt.Sprintf("SQL Server instance %s is running again", where)})
		}

		// SQL Agent cannot run without its engine, so while the engine is not running the
		// agent's state says nothing new and is not evaluated. Otherwise one stopped instance
		// would raise two alerts and two incidents.
		if inst.State == protocol.SqlStateRunning {
			switch edge(key+":agent", inst.AgentState) {
			case wentDown:
				alerts = append(alerts, Alert{notification.AlertSqlAgentStopped, scopeMachine, display, subject,
					notificationService.AlertTitle(display, "SQL Agent "+inst.AgentState+": "+inst.Name),
					fmt.Sprintf("SQL Server Agent for %s is %s. Scheduled jobs, including backups, are not running.", where, strings.ToLower(inst.AgentState))})
			case cameUp:
				alerts = append(alerts, Alert{notification.AlertSqlAgentStarted, scopeMachine, display, subject,
					notificationService.AlertTitle(display, "SQL Agent Running: "+inst.Name),
					fmt.Sprintf("SQL Server Agent for %s is running again", where)})
			}
		}
	}

	if cluster != nil {
		for _, n := range cluster.Nodes {
			subject := "cluster:" + cluster.Name + "/" + n.Name
			switch edge(fmt.Sprintf("%d:%s", projectID, subject), nodeState(n.State)) {
			case wentDown:
				alerts = append(alerts, Alert{notification.AlertClusterNodeDown, 0, n.Name, subject,
					notificationService.AlertTitle(n.Name, "Cluster Node Down"),
					fmt.Sprintf("%s is down in failover cluster %s", n.Name, cluster.Name)})
			case cameUp:
				alerts = append(alerts, Alert{notification.AlertClusterNodeUp, 0, n.Name, subject,
					notificationService.AlertTitle(n.Name, "Cluster Node Up"),
					fmt.Sprintf("%s is back up in failover cluster %s", n.Name, cluster.Name)})
			}
		}
		for _, ag := range cluster.AvailabilityGroups {
			key := fmt.Sprintf("%d:ag:%s/%s", projectID, cluster.Name, ag.Name)
			if from, moved := ownerChanged(key, ag.PrimaryNode); moved {
				alerts = append(alerts, Alert{notification.AlertSqlAgFailover, 0, ag.Name, "",
					notificationService.AlertTitle(ag.Name, "AG Failover"),
					fmt.Sprintf("Availability group %s primary moved from %s to %s", ag.Name, from, ag.PrimaryNode)})
			}
		}
	}

	return alerts
}

func instanceLabel(name, display string) string {
	if strings.EqualFold(name, "MSSQLSERVER") {
		return display + " (default instance)"
	}
	return display + `\` + name
}

type change int

const (
	unchanged change = iota
	wentDown
	cameUp
)

// edge records a state for key and reports whether it crossed between up and down.
// Transitional and unknown states are ignored, so a restart seen mid-way does not count.
func edge(key, state string) change {
	var isDown bool
	switch state {
	case protocol.SqlStateStopped, protocol.SqlStateFailed:
		isDown = true
	case protocol.SqlStateRunning:
		isDown = false
	default:
		return unchanged
	}
	prev, known := down[key]
	down[key] = isDown
	if !known || prev == isDown {
		return unchanged
	}
	if isDown {
		return wentDown
	}
	return cameUp
}

// ownerChanged records the owner for key and reports the previous one when it changed.
func ownerChanged(key, owner string) (string, bool) {
	if owner == "" {
		return "", false
	}
	prev, known := owners[key]
	owners[key] = owner
	if !known || strings.EqualFold(prev, owner) {
		return "", false
	}
	return prev, true
}

// nodeState maps a cluster node state onto the edge vocabulary. Paused and Joining are
// deliberate or transitional, not outages.
func nodeState(s string) string {
	switch s {
	case "Up":
		return protocol.SqlStateRunning
	case "Down":
		return protocol.SqlStateStopped
	default:
		return protocol.SqlStateUnknown
	}
}
