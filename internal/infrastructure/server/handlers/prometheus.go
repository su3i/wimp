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

const (
	// defaultProbeTimeoutSeconds is how long a health check probe is allowed to take
	// before it counts as a failure. It is deliberately generous: a probe that times out
	// is recorded as probe_success=0, which is indistinguishable from an endpoint that
	// refused the connection, so a tight timeout turns "the site is slow" into "the site
	// is down". Degraded-but-responding is caught by the slow-response alert instead
	// (see internal/application/monitor), which needs the probe to actually finish.
	defaultProbeTimeoutSeconds = 30
	// minProbeTimeoutSeconds floors the value derived below, so an aggressively short
	// health check interval can't shrink the timeout to something no real site can meet.
	minProbeTimeoutSeconds = 5
	// probeTimeoutHeadroomSeconds is kept between the timeout and the scrape interval.
	// Prometheus rejects a scrape timeout larger than its interval, and a timeout equal
	// to the interval leaves no room for the result to be recorded before the next scrape.
	probeTimeoutHeadroomSeconds = 5
)

// probeTimeoutSeconds resolves the per-probe timeout for an application whose health check
// runs every intervalSeconds.
func probeTimeoutSeconds(intervalSeconds int) int {
	timeout := config.Alerts().HealthCheckTimeoutSeconds
	if timeout <= 0 {
		timeout = defaultProbeTimeoutSeconds
	}
	if ceiling := intervalSeconds - probeTimeoutHeadroomSeconds; timeout > ceiling {
		timeout = ceiling
	}
	if timeout < minProbeTimeoutSeconds {
		timeout = minProbeTimeoutSeconds
	}
	return timeout
}

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
		// Terminal rows keep their last-known IPs, and that IP is usually still a live
		// box - it just answers for a different machine row now (reassigned) or is on its
		// way out (deleting). Leaving them here would have Prometheus keep scraping that
		// host under the retired machine_id, so the project the host left would go on
		// charting live metrics for a machine it no longer owns.
		if m.Status.Terminal() {
			continue
		}
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
				"__scrape_timeout__":  fmt.Sprintf("%ds", probeTimeoutSeconds(interval)),
			},
		})
	}

	c.JSON(http.StatusOK, targets)
}
