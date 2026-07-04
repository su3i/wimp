package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/su3i/wimp/internal/application/notification"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/monitor"
	notificationDomain "github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/infrastructure/database"
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

// ── Alert checker ─────────────────────────────────────────────────────────────

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func queryInstant(prometheusUrl, query string) (*promResponse, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", prometheusUrl, url.QueryEscape(query))
	resp, err := http.Get(u) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
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
	pr, err := queryInstant(prometheusUrl, `probe_success{job="blackbox_http"}`)
	if err != nil {
		log.Printf("monitor checker: prometheus query failed: %v", err)
		return
	}
	if pr.Status != "success" {
		return
	}

	repo := database.NewMonitorRepository(dbCfg)

	for _, r := range pr.Data.Result {
		idStr, ok := r.Metric["monitor_id"]
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

		m, err := repo.FindByID(uint(id))
		if err != nil || !m.Enabled {
			continue
		}

		if val == 0 {
			failures := m.ConsecutiveFailures + 1
			alertFired := m.AlertFired
			if failures >= 2 && !alertFired {
				notification.Emit(
					0,
					notificationDomain.LevelCritical,
					notificationDomain.CategoryService,
					fmt.Sprintf("Monitor Down: %s", m.Name),
					fmt.Sprintf("Endpoint %s is not responding", m.URL),
					dbCfg,
				)
				alertFired = true
			}
			_ = repo.UpdateCheckState(uint(id), failures, alertFired)
		} else {
			if m.AlertFired {
				notification.Emit(
					0,
					notificationDomain.LevelInfo,
					notificationDomain.CategoryService,
					fmt.Sprintf("Monitor Recovered: %s", m.Name),
					fmt.Sprintf("Endpoint %s is responding again", m.URL),
					dbCfg,
				)
			}
			_ = repo.UpdateCheckState(uint(id), 0, false)
		}
	}
}
