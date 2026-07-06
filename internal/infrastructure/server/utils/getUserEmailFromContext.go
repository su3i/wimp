package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func GetUsernameFromContext(c *gin.Context) (*string, error) {
	val, exists := c.Get("username")

	if !exists {
		return nil, errors.New("failed to retrieve username from context")
	}

	username, ok := val.(string)

	if !ok {
		return nil, errors.New("invalid username type")
	}

	return &username, nil
}
