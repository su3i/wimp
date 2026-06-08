package handlers

import (
	"bytes"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type bootstrapData struct {
	ControlPlaneUrl   string
	RegistrationToken string
	MachineId         uint
	LokiHost          string
	LokiPort          string
}

func Bootstrap(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "# Error: missing token query parameter\nexit 1")
		return
	}

	repo := database.NewMachineRepository(config.Database())

	m, err := repo.FindOneByToken(token)
	if err != nil || m == nil {
		c.String(http.StatusUnauthorized, "# Error: invalid token\nexit 1")
		return
	}

	if time.Now().After(m.TokenExpiresAt) {
		c.String(http.StatusUnauthorized, "# Error: token has expired\nexit 1")
		return
	}

	scriptBytes, err := os.ReadFile("./data/bootstrap.ps1")
	if err != nil {
		c.String(http.StatusInternalServerError, "# Error: bootstrap script unavailable\nexit 1")
		return
	}

	tmpl, err := template.New("bootstrap").Parse(string(scriptBytes))
	if err != nil {
		c.String(http.StatusInternalServerError, "# Error: bootstrap script parse error\nexit 1")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, bootstrapData{
		ControlPlaneUrl:   config.Common().AppUrl,
		RegistrationToken: m.Token,
		MachineId:         m.ID,
		LokiHost:          config.Common().LokiHost,
		LokiPort:          config.Common().LokiPort,
	}); err != nil {
		c.String(http.StatusInternalServerError, "# Error: bootstrap script render error\nexit 1")
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, buf.String())
}

func Uninstall(c *gin.Context) {
	scriptBytes, err := os.ReadFile("./data/cleanup.ps1")
	if err != nil {
		c.String(http.StatusInternalServerError, "# Error: cleanup script unavailable\nexit 1")
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, string(scriptBytes))
}
