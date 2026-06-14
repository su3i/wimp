package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	machineService "github.com/su3i/wimp/internal/application/machine"
	siteService "github.com/su3i/wimp/internal/application/site"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

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
