package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/su3i/wimp/internal/application/metadata"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

var Languages = []map[string]string{
	{"name": "English", "code": "EN", "default": "true"},
}

func SupportedLanguages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":   "success",
		"languages": Languages,
	})
	return
}

func SetLanguagePreference(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Validation failed.",
			"errors":  utils.FormatValidationErrors(err),
		})
		return
	}

	var isValid bool
	for _, lang := range Languages {
		if lang["code"] == req.Code {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported language code",
		})
		return
	}

	if err := metadata.SetLanguage(req.Code, config.Database()); err != nil {
		log.Printf("Error updating language: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update language",
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Language updated successfully",
		"code":    req.Code,
	})
}

func RetrieveLanguagePreference(c *gin.Context) {
	language, err := metadata.RetrieveLanguage(config.Database())

	if err != nil || *language == "" {
		for _, lang := range Languages {
			if lang["default"] == "true" {
				c.JSON(http.StatusOK, gin.H{
					"message":  "success",
					"language": lang["code"],
				})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"error": errors.New("Failed to get language preference"),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"language": language,
	})
	return
}
