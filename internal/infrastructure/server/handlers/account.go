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
	// Parse the request body
	var req struct {
		Name string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role string `json:"role" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors": utils.FormatValidationErrors(err),
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

	// Create account
	_account, err := accountService.NewAccount(req.Name, req.Email, req.Password, role, config.Database())

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

	// Retrieve accounts
	_accounts, err := accountService.RetrieveAccounts(config.Database())

	if err != nil {
		log.Printf("Error retrieving account: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"accounts": accountDomain.ToAccountDTOs(_accounts),
	})
}

func RetrieveAccountByEmail(c *gin.Context) {
	// Get email from query params
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required query parameter: email",
		})
		return
	}

	// Retrieve account
	_account, err := accountService.RetrieveAccount(email, config.Database())

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
		"message": "success",
		"account": accountDomain.ToAccountDTO(_account),
		"security_level": accountDomain.GetSecurityLevel(*_account),
	})
}

func UpdateAccount(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required query parameter: email",
		})
		return
	}

	var req struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors": utils.FormatValidationErrors(err),
		})
		return
	}

	_account, err := accountService.UpdateAccount(email, &req.Name, &req.Email, config.Database())

	if err != nil {
		log.Printf("Error updating account: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"account": _account,
		"security_level": accountDomain.GetSecurityLevel(*_account),
	})
}