package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	notificationService "github.com/su3i/wimp/internal/application/notification"
	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func DashboardStats(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	projectKey := c.Param("key")
	cfg := config.Database()

	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	db := database.GetDB(cfg)

	var machinesCount int64
	if err := db.Table("machines").Where("project_id = ? AND deleted_at IS NULL", proj.ID).Count(&machinesCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var applicationsCount int64
	if err := db.Table("applications").Where("project_id = ? AND deleted_at IS NULL", proj.ID).Count(&applicationsCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	unreadCount, err := notificationService.UnreadCount(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"machines_count":       machinesCount,
		"applications_count":   applicationsCount,
		"unread_notifications": unreadCount,
	})
}
