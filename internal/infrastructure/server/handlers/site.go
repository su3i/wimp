package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	machineService "github.com/su3i/wimp/internal/application/machine"
	siteService "github.com/su3i/wimp/internal/application/site"
	"github.com/su3i/wimp/internal/cache"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

var siteActionPendingState = map[string]string{
	"start":   "Starting",
	"stop":    "Stopping",
	"restart": "Restarting",
}

func RetrieveSites(c *gin.Context) {
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

	if _, _, err := machineService.GetBootstrapToken(uint(machineID), projectKey, "", "", config.Database()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	page, perPage, status := utils.ParsePageQuery(c)
	sites, total, err := siteService.RetrieveByMachineID(uint(machineID), page, perPage, status, config.Database())
	if err != nil {
		log.Printf("Error retrieving sites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"sites":    sites,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

func SiteCommand(action string) gin.HandlerFunc {
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

		siteID, err := strconv.ParseUint(c.Param("siteId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
			return
		}

		if _, _, err := machineService.GetBootstrapToken(uint(machineID), projectKey, "", "", config.Database()); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
			return
		}

		site, err := siteService.FindOneByID(uint(siteID), config.Database())
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
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
			TargetType: "site",
			Target:     site.Name,
		})
		msg, _ := json.Marshal(protocol.Message{
			Type:    protocol.TypeCommand,
			Payload: payload,
		})

		if err := hub.Get().Send(uint(machineID), msg); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send command"})
			return
		}

		if pendingState, ok := siteActionPendingState[action]; ok {
			cache.SetSitePending(uint(siteID), uint(machineID), pendingState)
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), commandTimeout)
		defer cancel()

		select {
		case result := <-ch:
			if result.Success {
				c.JSON(http.StatusOK, gin.H{"message": "success", "output": result.Output})
			} else {
				cache.InvalidateSite(uint(siteID))
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": result.Error, "output": result.Output})
			}
		case <-ctx.Done():
			cache.InvalidateSite(uint(siteID))
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "command timed out"})
		}
	}
}
