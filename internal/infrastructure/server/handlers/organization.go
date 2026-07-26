package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	organizationService "github.com/su3i/wimp/internal/application/organization"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

// NewOrganization is intentionally public, but only until the "default" organization
// has been created once - this is the first-run setup step, not an ongoing public API.
// Without this gate, anyone who can reach the control plane could recreate or squat the
// organization at any time.
func NewOrganization(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required"`
		Scope string `json:"scope" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	if existing, err := organizationService.RetrieveOrganization("default", config.Database()); err == nil && existing != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "organization already set up",
		})
		return
	}

	_organization, err := organizationService.NewOrganization(req.Name, "default", req.Scope, config.Database())

	if err != nil {
		log.Printf("Error creating organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "success",
		"organization": _organization,
	})
}

func RetrieveOrganization(c *gin.Context) {
	key := "default"

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
		"message":      "success",
		"organization": _organization,
	})
	return
}

func UpdateOrganization(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainOrg, authorizationDomain.Organization, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		Name  string `json:"name,omitempty"`
		Scope string `json:"scope,omitempty"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	_organization, err := organizationService.UpdateOrganization(&req.Name, "default", &req.Scope, config.Database())

	if err != nil {
		log.Printf("Error updating organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "success",
		"organization": _organization,
	})
	return
}
