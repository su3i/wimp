package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func GetUserEmailFromContext(c *gin.Context) (*string, error) {
	val, exists := c.Get("email")

	if !exists {
		return nil, errors.New("failed to retrieve email from context")
	}

	email, ok := val.(string)

	if !ok {
		return nil, errors.New("invalid email type")
	}

	return &email, nil
}