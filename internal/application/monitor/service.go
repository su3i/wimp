package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/su3i/wimp/internal/application/notification"
	"github.com/su3i/wimp/internal/config"
	applicationDomain "github.com/su3i/wimp/internal/domain/application"
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

// consecutiveBreachesBeforeAlert is how many checks in a row must agree before either the
// down or the slow alert fires, so a single unlucky probe doesn't page anyone.
const consecutiveBreachesBeforeAlert = 2

// defaultSlowThresholdSeconds is the probe duration above which a responding endpoint
// counts as degraded. Well under the probe timeout on purpose: a site that crosses this
// is still answering, and the whole point of the slow alert is to catch it while it is.
const defaultSlowThresholdSeconds = 5

// queryByApplication runs an instant query and returns its samples keyed by the
// application_id label, dropping anything unlabelled or unparseable.
func queryByApplication(prometheusUrl, query string) (map[uint]float64, error) {
	pr, err := prometheus.QueryInstant(prometheusUrl, query)
	if err != nil {
		return nil, err
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus returned status %q", pr.Status)
	}

	out := make(map[uint]float64, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		idStr, ok := r.Metric["application_id"]
		if !ok {
			continue
		}
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		// Prometheus encodes sample values as JSON strings, not numbers.
		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		out[uint(id)] = val
	}
	return out, nil
}

func slowThresholdSeconds() float64 {
	if configured := config.Alerts().ThresholdHealthCheckSlowSeconds; configured > 0 {
		return float64(configured)
	}
	return defaultSlowThresholdSeconds
}

func runCheck(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	success, err := queryByApplication(prometheusUrl, `probe_success{job="blackbox_http"}`)
	if err != nil {
		log.Printf("monitor checker: probe_success query failed: %v", err)
		return
	}

	// A missing or failed duration query only costs the slow check; the up/down check
	// below still runs, since that is the more important of the two.
	durations, err := queryByApplication(prometheusUrl, `probe_duration_seconds{job="blackbox_http"}`)
	if err != nil {
		log.Printf("monitor checker: probe_duration_seconds query failed: %v", err)
	}

	appRepo := database.NewApplicationRepository(dbCfg)
	threshold := slowThresholdSeconds()

	for id, val := range success {
		app, err := appRepo.FindOneByID(id)
		if err != nil || app == nil || app.HealthCheckURL == nil {
			continue
		}

		if val == 0 {
			checkDown(app, appRepo, dbCfg)
			continue
		}

		checkUp(app, appRepo, dbCfg)

		duration, ok := durations[id]
		if !ok {
			continue
		}
		checkSlow(app, duration, threshold, appRepo, dbCfg)
	}
}

// checkDown handles a probe that did not succeed: no answer, a non-2xx status, or a
// response that ran past the probe timeout.
func checkDown(app *applicationDomain.Application, appRepo applicationDomain.ApplicationRepository, dbCfg *config.DatabaseConfig) {
	failures := app.ConsecutiveFailures + 1
	alertFired := app.AlertFired
	if failures >= consecutiveBreachesBeforeAlert && !alertFired {
		notification.EmitAlert(
			notificationDomain.AlertHealthCheckDown,
			app.ProjectID, 0, app.Name,
			notification.AlertTitle(app.Name, "Health Check Failing"),
			fmt.Sprintf("Endpoint %s is not responding", *app.HealthCheckURL),
			dbCfg,
		)
		alertFired = true
	}
	_ = appRepo.UpdateCheckState(app.ID, failures, alertFired)

	// A site that is down is not separately "slow" - clear that state so it doesn't
	// emit a spurious slow-recovered alert the moment it comes back.
	if app.ConsecutiveSlow != 0 || app.SlowAlertFired {
		_ = appRepo.UpdateSlowState(app.ID, 0, false)
	}
}

func checkUp(app *applicationDomain.Application, appRepo applicationDomain.ApplicationRepository, dbCfg *config.DatabaseConfig) {
	if app.AlertFired {
		notification.EmitAlert(
			notificationDomain.AlertHealthCheckUp,
			app.ProjectID, 0, app.Name,
			notification.AlertTitle(app.Name, "Health Check recovered"),
			fmt.Sprintf("Endpoint %s is responding again", *app.HealthCheckURL),
			dbCfg,
		)
	}
	_ = appRepo.UpdateCheckState(app.ID, 0, false)
}

// checkSlow edge-triggers the degraded-response alert for an endpoint that is answering.
// Only reached when the probe succeeded, so "slow" here always means the site is up and
// serving, just taking too long about it.
func checkSlow(app *applicationDomain.Application, duration, threshold float64, appRepo applicationDomain.ApplicationRepository, dbCfg *config.DatabaseConfig) {
	if duration > threshold {
		slow := app.ConsecutiveSlow + 1
		fired := app.SlowAlertFired
		if slow >= consecutiveBreachesBeforeAlert && !fired {
			notification.EmitAlert(
				notificationDomain.AlertHealthCheckSlow,
				app.ProjectID, 0, app.Name,
				notification.AlertTitle(app.Name, "Site Slow"),
				fmt.Sprintf("Endpoint %s is responding in %.1fs (threshold %.1fs). The site is up but degraded.",
					*app.HealthCheckURL, duration, threshold),
				dbCfg,
			)
			fired = true
		}
		_ = appRepo.UpdateSlowState(app.ID, slow, fired)
		return
	}

	if app.SlowAlertFired {
		notification.EmitAlert(
			notificationDomain.AlertHealthCheckFast,
			app.ProjectID, 0, app.Name,
			notification.AlertTitle(app.Name, "Site Slow recovered"),
			fmt.Sprintf("Endpoint %s is back to %.1fs (threshold %.1fs)", *app.HealthCheckURL, duration, threshold),
			dbCfg,
		)
	}
	if app.ConsecutiveSlow != 0 || app.SlowAlertFired {
		_ = appRepo.UpdateSlowState(app.ID, 0, false)
	}
}
