package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

const windowsExporterPort = "9182"

type prometheusTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// PrometheusTargets returns all registered machines with IPs in Prometheus
// HTTP service discovery format. Prometheus polls this endpoint to discover
// scrape targets dynamically as machines bootstrap and connect.
func PrometheusTargets(c *gin.Context) {
	machines, err := database.NewMachineRepository(config.Database()).FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targets := make([]prometheusTarget, 0, len(*machines))
	for _, m := range *machines {
		ip := firstIPv4(m.IPs)
		if ip == "" {
			continue
		}
		targets = append(targets, prometheusTarget{
			Targets: []string{net.JoinHostPort(ip, windowsExporterPort)},
			Labels: map[string]string{
				"machine_id": strconv.Itoa(int(m.ID)),
				"hostname":   m.Hostname,
				"status":     string(m.Status),
			},
		})
	}

	c.JSON(http.StatusOK, targets)
}

func firstIPv4(ips []string) string {
	for _, raw := range ips {
		ip := net.ParseIP(raw)
		if ip != nil && ip.To4() != nil {
			return raw
		}
	}
	return ""
}

// PrometheusMonitorTargets returns blackbox_exporter HTTP SD targets for all
// applications that have a health check URL configured.
func PrometheusMonitorTargets(c *gin.Context) {
	apps, err := database.NewApplicationRepository(config.Database()).FindAllWithHealthCheck()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targets := make([]prometheusTarget, 0, len(apps))
	for _, app := range apps {
		if app.HealthCheckURL == nil || *app.HealthCheckURL == "" {
			continue
		}
		interval := app.HealthCheckIntervalSeconds
		if interval <= 0 {
			interval = 60
		}
		targets = append(targets, prometheusTarget{
			Targets: []string{*app.HealthCheckURL},
			Labels: map[string]string{
				"application_id":      strconv.Itoa(int(app.ID)),
				"application_name":    app.Name,
				"__scrape_interval__": fmt.Sprintf("%ds", interval),
				"__scrape_timeout__":  "10s",
			},
		})
	}

	c.JSON(http.StatusOK, targets)
}
