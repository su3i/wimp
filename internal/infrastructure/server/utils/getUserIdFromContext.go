package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func GetUserIdFromContext(c *gin.Context) (*string, error) {
	val, exists := c.Get("userId")

	if !exists {
		return nil, errors.New("failed to retrieve userId from context")
	}

	userId, ok := val.(string)

	if !ok {
		return nil, errors.New("invalid userid type")
	}

	return &userId, nil
}