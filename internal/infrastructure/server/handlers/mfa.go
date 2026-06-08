package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	accountService "github.com/su3i/wimp/internal/application/account"
	"github.com/su3i/wimp/internal/application/mfa"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func RetrieveTotpURI(c *gin.Context) {
	// Parse the request body
	var req struct {
		Email string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors": utils.FormatValidationErrors(err),
		})
		return
	}

	// Retrieve account
	_account, err := accountService.RetrieveAccountWithPassword(req.Email, req.Password, config.Database())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if _account == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid account.",
		})
		return
	}

	// Retrieve TOTP URI
	uri, err := mfa.RetrieveTotpURI(req.Email, _account.MFASecret)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve TOTP URI",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "success",
		"uri": uri,
	  })
	return
}

// Confirm and Enable MFA
func ConfirmMFA(c *gin.Context) {
	// Parse the request body
	var req struct {
		Email string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		Code string `json:"code" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors": utils.FormatValidationErrors(err),
		})
		return
	}

	// Retrieve account
	_account, err := accountService.RetrieveAccountWithPassword(req.Email, req.Password, config.Database())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if _account == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid account.",
		})
		return
	}

	codeUint64, err := strconv.ParseUint(req.Code, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mfa code format"})
		return
	}

	code := uint32(codeUint64)

	// Confirm and enable MFA
	isCodeValid := mfa.VerifyTOTP(_account.MFASecret, code, time.Now())

	if !isCodeValid {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid TOTP code.",
		})
		return
	}

	accountService.EnableTOTP(req.Email, config.Database())

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
	return
}