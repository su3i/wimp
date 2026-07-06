// Package metrics periodically queries Prometheus for per-machine CPU/memory/disk and
// edge-triggers threshold alerts. Mirrors the same shape as internal/application/
// monitor's health-check poller (same StartChecker/runCheck pattern, same
// "edge-triggered, in-memory breach state, resets on restart" tradeoff).
package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/su3i/wimp/internal/application/notification"
	"github.com/su3i/wimp/internal/config"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	notificationDomain "github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/prometheus"
)

const checkInterval = 60 * time.Second

// Hardcoded fallbacks used when no threshold is configured via internal/config/alerts.go.
const (
	defaultHighCPUPercent    = 90
	defaultHighMemoryPercent = 90
	defaultLowDiskPercent    = 10 // alert when free space drops below this
)

type breachKey struct {
	machineID uint
	alertType notificationDomain.AlertType
}

var (
	breachMu    sync.Mutex
	breachState = map[breachKey]bool{}
)

func StartChecker(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	if prometheusUrl == "" {
		log.Println("metrics checker: PROMETHEUSURL not set, threshold checking disabled")
		return
	}
	go func() {
		time.Sleep(60 * time.Second) // let Prometheus warm up, same as the health-check poller
		for {
			runCheck(dbCfg, prometheusUrl)
			time.Sleep(checkInterval)
		}
	}()
}

func runCheck(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	machines, err := database.NewMachineRepository(dbCfg).FindAll()
	if err != nil || machines == nil {
		return
	}

	online := map[uint]machineDomain.Machine{}
	for _, m := range *machines {
		if m.Status == machineDomain.Online {
			online[m.ID] = m
		}
	}
	if len(online) == 0 {
		return
	}

	a := config.Alerts()

	checkThreshold(dbCfg, prometheusUrl, online,
		notificationDomain.AlertHighCPU, notificationDomain.AlertHighCPURecovered,
		`100 - (avg by (machine_id) (rate(windows_cpu_time_total{mode="idle"}[5m])) * 100)`,
		thresholdOrDefault(a.ThresholdHighCPUPercent, defaultHighCPUPercent),
		false, "CPU usage")

	checkThreshold(dbCfg, prometheusUrl, online,
		notificationDomain.AlertHighMemory, notificationDomain.AlertHighMemoryRecovered,
		`100 - (windows_memory_physical_free_bytes / windows_memory_physical_total_bytes * 100)`,
		thresholdOrDefault(a.ThresholdHighMemoryPercent, defaultHighMemoryPercent),
		false, "memory usage")

	checkThreshold(dbCfg, prometheusUrl, online,
		notificationDomain.AlertLowDisk, notificationDomain.AlertLowDiskRecovered,
		`min by (machine_id) (windows_logical_disk_free_bytes{volume=~"[A-Z]:.*"} / windows_logical_disk_size_bytes{volume=~"[A-Z]:.*"} * 100)`,
		thresholdOrDefault(a.ThresholdLowDiskPercent, defaultLowDiskPercent),
		true, "free disk space")
}

// checkThreshold queries a per-machine PromQL result and edge-triggers a breach/recovery
// alert. If invert is true, a breach is "value below threshold" (e.g. low disk space);
// otherwise it's "value above threshold" (e.g. high CPU).
func checkThreshold(dbCfg *config.DatabaseConfig, prometheusUrl string, online map[uint]machineDomain.Machine,
	alertType, recoveredType notificationDomain.AlertType, query string, threshold float64, invert bool, label string) {

	pr, err := prometheus.QueryInstant(prometheusUrl, query)
	if err != nil {
		log.Printf("metrics checker: prometheus query failed (%s): %v", alertType, err)
		return
	}
	if pr.Status != "success" {
		return
	}

	for _, r := range pr.Data.Result {
		idStr, ok := r.Metric["machine_id"]
		if !ok {
			continue
		}
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		m, ok := online[uint(id)]
		if !ok {
			continue
		}

		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		breached := val > threshold
		if invert {
			breached = val < threshold
		}

		key := breachKey{machineID: m.ID, alertType: alertType}
		wasBreached := getBreach(key)

		if breached && !wasBreached {
			setBreach(key, true)
			notification.EmitAlert(alertType, m.ProjectID, m.ID, m.Hostname,
				fmt.Sprintf("%s — %s", m.Hostname, titleFor(alertType)),
				fmt.Sprintf("%s %s is %.1f%% (threshold %.1f%%)", m.Hostname, label, val, threshold),
				dbCfg)
		} else if !breached && wasBreached {
			setBreach(key, false)
			notification.EmitAlert(recoveredType, m.ProjectID, m.ID, m.Hostname,
				fmt.Sprintf("%s — %s Recovered", m.Hostname, titleFor(alertType)),
				fmt.Sprintf("%s %s is back to %.1f%%", m.Hostname, label, val),
				dbCfg)
		}
	}
}

func titleFor(alertType notificationDomain.AlertType) string {
	switch alertType {
	case notificationDomain.AlertHighCPU:
		return "High CPU"
	case notificationDomain.AlertHighMemory:
		return "High Memory"
	case notificationDomain.AlertLowDisk:
		return "Low Disk Space"
	default:
		return string(alertType)
	}
}

func thresholdOrDefault(configured, fallback int) float64 {
	if configured > 0 {
		return float64(configured)
	}
	return float64(fallback)
}

func getBreach(key breachKey) bool {
	breachMu.Lock()
	defer breachMu.Unlock()
	return breachState[key]
}

func setBreach(key breachKey, v bool) {
	breachMu.Lock()
	defer breachMu.Unlock()
	if v {
		breachState[key] = true
	} else {
		delete(breachState, key)
	}
}
