package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/config"
)

func RetrieveConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enforce_mfa": config.Common().EnforceMfa,
	})
	return
}