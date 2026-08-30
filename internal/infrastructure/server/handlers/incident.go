package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	incidentService "github.com/su3i/wimp/internal/application/incident"
	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	incidentDomain "github.com/su3i/wimp/internal/domain/incident"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

// ListIncidents returns a project's incident timeline, newest first.
func ListIncidents(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	proj, err := projectService.RetrieveProject(projectKey, config.Database())
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	page, perPage, status := utils.ParsePageQuery(c)

	// Anything other than the two real states is treated as "no filter" rather than as an
	// error - the UI's "All" tab sends no status at all.
	var filter incidentDomain.Status
	switch incidentDomain.Status(status) {
	case incidentDomain.StatusOpen:
		filter = incidentDomain.StatusOpen
	case incidentDomain.StatusResolved:
		filter = incidentDomain.StatusResolved
	}

	incidents, total, err := incidentService.List(proj.ID, filter, page, perPage, config.Database())
	if err != nil {
		log.Printf("Error listing incidents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	counts, err := incidentService.Counts(proj.ID, config.Database())
	if err != nil {
		log.Printf("Error counting incidents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "success",
		"incidents": incidents,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"counts":    gin.H{"open": counts.Open, "resolved": counts.Resolved},
	})
}
