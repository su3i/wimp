package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	organizationService "github.com/su3i/wimp/internal/application/organization"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func NewOrganization(c *gin.Context) {
	// Parse the request body
	var req struct {
		Name string `json:"name" binding:"required"`
		Scope string `json:"scope" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors": utils.FormatValidationErrors(err),
		})
		return
	}

	// Create organization
	_organization, err := organizationService.NewOrganization(req.Name, "default", req.Scope, config.Database())

	if err != nil {
		log.Printf("Error creating organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "success",
		"organization": _organization,
	  })
}

func RetrieveOrganization(c *gin.Context) {
	key := "default" // Default organization key

	// Retrieve organization
	_organization, err := organizationService.RetrieveOrganization(key, config.Database())

	if err != nil {
		log.Printf("Error retrieving organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if _organization == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Not Found.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"organization": _organization,
	})
	return
}

func UpdateOrganization (c *gin.Context) {
	// Parse the request body
	var req struct {
		Name string `json:"name,omitempty"`
		Scope string `json:"scope,omitempty"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors": utils.FormatValidationErrors(err),
		})
		return
	}

	// Update organization
	_organization, err := organizationService.UpdateOrganization(&req.Name, "default", &req.Scope, config.Database())

	if err != nil {
		log.Printf("Error updating organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "success",
		"organization": _organization,
	})
	return
}
