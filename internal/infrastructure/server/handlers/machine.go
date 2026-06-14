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
	"github.com/su3i/wimp/internal/hub"
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

	downloadCmd, runCmd, err := machineService.GetBootstrapToken(m.ID, projectKey, config.Common().AppUrl, config.Common().AppEnv, config.Database())
	if err != nil {
		log.Printf("Error building bootstrap command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":          "success",
		"machine":          m,
		"download_command": downloadCmd,
		"run_command":      runCmd,
	})
}

func RetrieveMachines(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	page, perPage, status := utils.ParsePageQuery(c)

	machines, total, err := machineService.RetrieveMachines(projectKey, page, perPage, status, config.Database())
	if err != nil {
		log.Printf("Error retrieving machines: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"machines": machines,
		"total":    total,
		"page":     page,
		"per_page": perPage,
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

	downloadCmd, runCmd, err := machineService.GetBootstrapToken(uint(id), projectKey, config.Common().AppUrl, config.Common().AppEnv, config.Database())
	if err != nil {
		log.Printf("Error getting bootstrap token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_command": downloadCmd,
		"run_command":      runCmd,
	})
}

func DeleteMachine(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseUint(c.Param("machineId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
		return
	}

	cmd, err := machineService.RequestDeletion(uint(id), projectKey, config.Common().AppUrl, config.Common().AppEnv, config.Database())
	if err != nil {
		log.Printf("Error requesting machine deletion: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// If the machine is not currently connected there is no disconnect event to
	// trigger the hard delete, so do it now.
	if !hub.Get().IsOnline(uint(id)) {
		if err := machineService.HardDelete(uint(id), config.Database()); err != nil {
			log.Printf("Error hard deleting offline machine: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "success", "uninstall_command": cmd})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "uninstall_command": cmd})
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
