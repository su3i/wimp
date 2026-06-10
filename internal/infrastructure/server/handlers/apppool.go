package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appPoolService "github.com/su3i/wimp/internal/application/apppool"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	machineService "github.com/su3i/wimp/internal/application/machine"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

const commandTimeout = 30 * time.Second

func RetrieveAppPools(c *gin.Context) {
	projectKey := c.Param("key")

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

	if _, err := machineService.GetBootstrapToken(uint(machineID), projectKey, "", "", config.Database()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	pools, err := appPoolService.RetrieveByMachineID(uint(machineID), config.Database())
	if err != nil {
		log.Printf("Error retrieving app pools: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "app_pools": pools})
}

func AppPoolCommand(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectKey := c.Param("key")

		allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
		if err != nil || !allow {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		machineID, err := strconv.ParseUint(c.Param("machineId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
			return
		}

		poolID, err := strconv.ParseUint(c.Param("poolId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool id"})
			return
		}

		if _, err := machineService.GetBootstrapToken(uint(machineID), projectKey, "", "", config.Database()); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
			return
		}

		pool, err := appPoolService.FindOneByID(uint(poolID), config.Database())
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "app pool not found"})
			return
		}

		if !hub.Get().IsOnline(uint(machineID)) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "machine is not connected"})
			return
		}

		cmdID := uuid.New().String()
		ch := hub.RegisterCommand(cmdID)
		defer hub.DeregisterCommand(cmdID)

		payload, _ := json.Marshal(protocol.CommandPayload{
			CommandID:  cmdID,
			Action:     action,
			TargetType: "app_pool",
			Target:     pool.Name,
		})
		msg, _ := json.Marshal(protocol.Message{
			Type:    protocol.TypeCommand,
			Payload: payload,
		})

		if err := hub.Get().Send(uint(machineID), msg); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send command"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), commandTimeout)
		defer cancel()

		select {
		case result := <-ch:
			if result.Success {
				c.JSON(http.StatusOK, gin.H{"message": "success", "output": result.Output})
			} else {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": result.Error, "output": result.Output})
			}
		case <-ctx.Done():
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "command timed out"})
		}
	}
}
