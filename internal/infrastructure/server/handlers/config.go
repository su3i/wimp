package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	settingService "github.com/su3i/wimp/internal/application/setting"
)

// RetrieveConfig exposes the handful of settings the frontend needs before a user is
// authenticated. Read through the settings layer rather than straight off the env, so a
// change made in the UI is in force immediately rather than at the next redeploy.
func RetrieveConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enforce_mfa": settingService.EnforceMfa(),
	})
}
