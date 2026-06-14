package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	applicationService "github.com/su3i/wimp/internal/application/application"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func NewApplication(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	app, err := applicationService.Create(req.Name, projectKey, config.Database())
	if err != nil {
		log.Printf("Error creating application: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "success", "application": app})
}

func RetrieveApplications(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	page, perPage, _ := utils.ParsePageQuery(c)
	apps, total, err := applicationService.RetrieveAll(projectKey, page, perPage, config.Database())
	if err != nil {
		log.Printf("Error retrieving applications: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "success",
		"applications": apps,
		"total":        total,
		"page":         page,
		"per_page":     perPage,
	})
}

func RetrieveApplication(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	detail, err := applicationService.GetDetail(uint(id), projectKey, config.Database())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "application": detail})
}

func AddAppPoolToApplication(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	appId, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	var req struct {
		AppPoolIDs []uint `json:"app_pool_ids"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	if err := applicationService.AddAppPools(uint(appId), req.AppPoolIDs, projectKey, config.Database()); err != nil {
		log.Printf("Error adding app pools to application: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func ListApplicationFiles(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	files, err := applicationService.ListApplicationFiles(c.Request.Context(), uint(id), projectKey, config.Database())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "files": files})
}

func DeleteApplication(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	id, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	if err := applicationService.Delete(uint(id), projectKey, config.Database()); err != nil {
		log.Printf("Error deleting application: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func UpdateAppPoolInApplication(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	appId, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	poolId, err := strconv.ParseUint(c.Param("poolId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool id"})
		return
	}

	var req struct {
		LogPath string `json:"log_path" binding:"required"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	if err := applicationService.UpdateAppPoolLogPath(uint(appId), uint(poolId), req.LogPath, projectKey, config.Database()); err != nil {
		log.Printf("Error updating app pool log path: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func ListApplicationAppPools(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	appId, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	page, perPage, status := utils.ParsePageQuery(c)
	pools, total, err := applicationService.ListAppPools(uint(appId), projectKey, page, perPage, status, config.Database())
	if err != nil {
		log.Printf("Error listing application app pools: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "success",
		"app_pools": pools,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
	})
}
