package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/su3i/wimp/internal/application/notification"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/monitor"
	notificationDomain "github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/prometheus"
)

func Create(projectID uint, name, rawURL string, intervalSeconds int, cfg *config.DatabaseConfig) (*monitor.Monitor, error) {
	m := &monitor.Monitor{
		ProjectID:       projectID,
		Name:            name,
		URL:             rawURL,
		IntervalSeconds: intervalSeconds,
		Enabled:         true,
	}
	return database.NewMonitorRepository(cfg).Create(m)
}

func List(projectKey string, cfg *config.DatabaseConfig) ([]monitor.Monitor, error) {
	return database.NewMonitorRepository(cfg).FindByProjectKey(projectKey)
}

func Delete(id uint, cfg *config.DatabaseConfig) error {
	return database.NewMonitorRepository(cfg).Delete(id)
}

func Update(id uint, name, rawURL string, intervalSeconds int, cfg *config.DatabaseConfig) (*monitor.Monitor, error) {
	repo := database.NewMonitorRepository(cfg)
	m, err := repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	m.Name = name
	m.URL = rawURL
	m.IntervalSeconds = intervalSeconds
	if err := repo.Update(m); err != nil {
		return nil, err
	}
	return m, nil
}

func AllForSD(cfg *config.DatabaseConfig) ([]monitor.Monitor, error) {
	return database.NewMonitorRepository(cfg).FindAll()
}

func StartChecker(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	if prometheusUrl == "" {
		log.Println("monitor checker: PROMETHEUSURL not set, alert checking disabled")
		return
	}
	go func() {
		// Initial delay so Prometheus has time to collect first probe results.
		time.Sleep(60 * time.Second)
		for {
			runCheck(dbCfg, prometheusUrl)
			time.Sleep(30 * time.Second)
		}
	}()
}

func runCheck(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	pr, err := prometheus.QueryInstant(prometheusUrl, `probe_success{job="blackbox_http"}`)
	if err != nil {
		log.Printf("monitor checker: prometheus query failed: %v", err)
		return
	}
	if pr.Status != "success" {
		return
	}

	appRepo := database.NewApplicationRepository(dbCfg)

	for _, r := range pr.Data.Result {
		idStr, ok := r.Metric["application_id"]
		if !ok {
			continue
		}
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}

		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, _ := strconv.ParseFloat(valStr, 64)

		app, err := appRepo.FindOneByID(uint(id))
		if err != nil || app == nil || app.HealthCheckURL == nil {
			continue
		}

		if val == 0 {
			failures := app.ConsecutiveFailures + 1
			alertFired := app.AlertFired
			if failures >= 2 && !alertFired {
				notification.EmitAlert(
					notificationDomain.AlertHealthCheckDown,
					app.ProjectID, 0, app.Name,
					notification.AlertTitle(app.Name, "Health Check Failing"),
					fmt.Sprintf("Endpoint %s is not responding", *app.HealthCheckURL),
					dbCfg,
				)
				alertFired = true
			}
			_ = appRepo.UpdateCheckState(uint(id), failures, alertFired)
		} else {
			if app.AlertFired {
				notification.EmitAlert(
					notificationDomain.AlertHealthCheckUp,
					app.ProjectID, 0, app.Name,
					notification.AlertTitle(app.Name, "Health Check recovered"),
					fmt.Sprintf("Endpoint %s is responding again", *app.HealthCheckURL),
					dbCfg,
				)
			}
			_ = appRepo.UpdateCheckState(uint(id), 0, false)
		}
	}
}
