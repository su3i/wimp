package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/su3i/wimp/internal/application/account"
	accountService "github.com/su3i/wimp/internal/application/account"
	"github.com/su3i/wimp/internal/application/authentication"
	"github.com/su3i/wimp/internal/application/mfa"
	"github.com/su3i/wimp/internal/config"
	authenticationDomain "github.com/su3i/wimp/internal/domain/authentication"
	"github.com/su3i/wimp/internal/infrastructure/cache"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	_account, err := account.RetrieveAccountWithPassword(req.Username, req.Password, config.Database())

	if err != nil || _account == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	if _account.MFAEnabled {
		challengeID := uuid.New().String()

		challengeKey := fmt.Sprintf("challenge-id-%s", challengeID)

		err = cache.GetCache().Set(challengeKey, req.Username, time.Hour)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "success",
			"mfa_required": _account.MFAEnabled,
			"challenge_id": challengeID,
		})
		return
	}

	auth, err := authentication.Login(req.Username, req.Password, config.Common(), config.Database())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "success",
		"access_token":  auth.AccessToken,
		"refresh_token": auth.RefreshToken,
	})
	return
}

func MFA(c *gin.Context) {
	var req struct {
		ChallengeID string `json:"challenge_id" binding:"required"`
		Code        string `json:"code" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	challengeKey := fmt.Sprintf("challenge-id-%s", req.ChallengeID)

	username, err := cache.GetCache().Get(challengeKey)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Please restart login flow.",
		})
		return
	}

	_account, err := accountService.RetrieveAccount(username, config.Database())

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

	isCodeValid := mfa.VerifyTOTP(_account.MFASecret, code, time.Now())

	if !isCodeValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid TOTP code.",
		})
		return
	}

	defer func() {
		_ = cache.GetCache().Delete(challengeKey)
	}()

	auth, err := authentication.LoginWithoutPassword(username, config.Common(), config.Database())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "success",
		"access_token":  auth.AccessToken,
		"refresh_token": auth.RefreshToken,
	})
	return
}

func RevokeToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	err := cache.GetCache().Delete(fmt.Sprintf("refresh-token-%s", authenticationDomain.HashRefreshToken(req.RefreshToken)))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
	return
}

func RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	authTokens, err := authentication.Refresh(req.RefreshToken, config.Common(), config.Database())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid or expired refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "success",
		"access_token":  authTokens.AccessToken,
		"refresh_token": authTokens.RefreshToken,
	})
}
