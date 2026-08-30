package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	incidentService "github.com/su3i/wimp/internal/application/incident"
	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

// resolveProject centralizes the lookup both incident handlers start with.
func resolveProject(c *gin.Context, action string) (uint, bool) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, action)
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return 0, false
	}

	proj, err := projectService.RetrieveProject(c.Param("key"), config.Database())
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return 0, false
	}
	return proj.ID, true
}

// ListIncidents returns a project's incident timeline: everything open first, newest
// first, then everything resolved within the retention window, newest first.
func ListIncidents(c *gin.Context) {
	projectID, ok := resolveProject(c, "read")
	if !ok {
		return
	}

	page, perPage, _ := utils.ParsePageQuery(c)

	incidents, total, err := incidentService.List(projectID, page, perPage, config.Database())
	if err != nil {
		log.Printf("Error listing incidents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	counts, err := incidentService.Counts(projectID, config.Database())
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
		// Lets the client tell "no more pages" apart from "nothing here", and label the
		// end of the feed with the window it covers.
		"window_days": int(incidentService.TimelineWindow.Hours() / 24),
		"counts":      gin.H{"open": counts.Open, "resolved": counts.Resolved},
	})
}

// ResolveIncident closes an open incident by hand.
func ResolveIncident(c *gin.Context) {
	projectID, ok := resolveProject(c, "write")
	if !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("incidentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident id"})
		return
	}

	// Best effort: the audit line reads better with a name on it, but a missing username
	// is no reason to refuse the resolution.
	var resolvedBy string
	if username, err := utils.GetUsernameFromContext(c); err == nil && username != nil {
		resolvedBy = *username
	}

	err = incidentService.ResolveManually(uint(id), projectID, resolvedBy, config.Database())
	switch {
	case errors.Is(err, incidentService.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
	case errors.Is(err, incidentService.ErrNotOpen):
		c.JSON(http.StatusConflict, gin.H{"error": "incident is already resolved"})
	case err != nil:
		log.Printf("Error resolving incident: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	}
}
