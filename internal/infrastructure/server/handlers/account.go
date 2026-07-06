package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	accountService "github.com/su3i/wimp/internal/application/account"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	"github.com/su3i/wimp/internal/config"
	accountDomain "github.com/su3i/wimp/internal/domain/account"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func NewAccount(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	role, err := accountDomain.NewAccountRole(req.Role)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "Invalid role.",
		})
		return
	}

	_account, err := accountService.NewAccount(req.Name, req.Username, req.Password, role, config.Database())

	if err != nil {
		log.Printf("Error creating account: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "success",
		"account": accountDomain.ToAccountDTO(_account),
	})
}

func RetrieveAccounts(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), "org", authorizationDomain.Organization, "read")

	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "forbidden",
		})
		return
	}

	_accounts, err := accountService.RetrieveAccounts(config.Database())

	if err != nil {
		log.Printf("Error retrieving account: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"accounts": accountDomain.ToAccountDTOs(_accounts),
	})
}

func RetrieveAccountByUsername(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required query parameter: username",
		})
		return
	}

	_account, err := accountService.RetrieveAccount(username, config.Database())

	if err != nil {
		log.Printf("Error retrieving account: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if _account == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Not Found.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "success",
		"account":        accountDomain.ToAccountDTO(_account),
		"security_level": accountDomain.GetSecurityLevel(*_account),
	})
}

func UpdateAccount(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required query parameter: username",
		})
		return
	}

	var req struct {
		Name     string `json:"name,omitempty"`
		Username string `json:"username,omitempty"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	_account, err := accountService.UpdateAccount(username, &req.Name, &req.Username, config.Database())

	if err != nil {
		log.Printf("Error updating account: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "success",
		"account":        _account,
		"security_level": accountDomain.GetSecurityLevel(*_account),
	})
}
