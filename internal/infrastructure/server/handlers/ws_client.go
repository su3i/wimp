package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/hub"
)

func ClientWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := validateJWT(token); err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("client ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	id := uuid.New().String()
	hub.Clients().Register(id, conn)
	defer hub.Clients().Deregister(id)

	// Keep connection alive; read and discard any client pings.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func validateJWT(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(config.Common().JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return errors.New("invalid token")
	}
	return nil
}
