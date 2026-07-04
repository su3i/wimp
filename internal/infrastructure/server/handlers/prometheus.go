package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	monitorService "github.com/su3i/wimp/internal/application/monitor"
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
// configured monitors. Prometheus polls this to discover what URLs to probe.
func PrometheusMonitorTargets(c *gin.Context) {
	monitors, err := monitorService.AllForSD(config.Database())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targets := make([]prometheusTarget, 0, len(monitors))
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}
		targets = append(targets, prometheusTarget{
			Targets: []string{m.URL},
			Labels: map[string]string{
				"monitor_id":            strconv.Itoa(int(m.ID)),
				"monitor_name":          m.Name,
				"__scrape_interval__":   fmt.Sprintf("%ds", m.IntervalSeconds),
				"__scrape_timeout__":    "10s",
			},
		})
	}

	c.JSON(http.StatusOK, targets)
}
