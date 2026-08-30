package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	settingService "github.com/su3i/wimp/internal/application/setting"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

// Settings are organization-wide rather than per-project, so they are gated on the org
// domain: reading needs any org role, changing needs write.
func authorizeSettings(c *gin.Context, action string) bool {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainOrg, authorizationDomain.Organization, action)
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

func RetrieveSettings(c *gin.Context) {
	if !authorizeSettings(c, "read") {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"settings": settingService.Current(),
		"levels":   settingService.Levels,
	})
}

func UpdateSettings(c *gin.Context) {
	if !authorizeSettings(c, "write") {
		return
	}

	var req struct {
		Changes []settingService.Change `json:"changes" binding:"required"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	if err := settingService.Apply(req.Changes, config.Database()); err != nil {
		if errors.Is(err, settingService.ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("Error updating settings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"settings": settingService.Current(),
	})
}
