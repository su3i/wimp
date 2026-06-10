package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	machineService "github.com/su3i/wimp/internal/application/machine"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func NewMachine(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	m, err := machineService.NewMachine(projectKey, config.Database())
	if err != nil {
		log.Printf("Error creating machine: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cmd, err := machineService.GetBootstrapToken(m.ID, projectKey, config.Common().AppUrl, config.Common().AppEnv, config.Database())
	if err != nil {
		log.Printf("Error building bootstrap command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":           "success",
		"machine":           m,
		"bootstrap_command": cmd,
	})
}

func RetrieveMachines(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	machines, err := machineService.RetrieveMachines(projectKey, config.Database())
	if err != nil {
		log.Printf("Error retrieving machines: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"machines": machines,
	})
}

func GetBootstrapToken(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	idParam := c.Param("machineId")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
		return
	}

	cmd, err := machineService.GetBootstrapToken(uint(id), projectKey, config.Common().AppUrl, config.Common().AppEnv, config.Database())
	if err != nil {
		log.Printf("Error getting bootstrap token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, cmd)
}

func GetUninstallCommand(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	idParam := c.Param("machineId")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
		return
	}

	cmd, err := machineService.GetUninstallCommand(uint(id), projectKey, config.Common().AppUrl, config.Common().AppEnv, config.Database())
	if err != nil {
		log.Printf("Error getting uninstall command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, cmd)
}
