package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/config"
)

func Health(c *gin.Context) {
	log.Print("Received health check request..")

	c.JSON(http.StatusOK, gin.H{
		"message": "Healthy",
		"version": "v0.0.0",
		"environment": config.Common().AppEnv,
	  })
}