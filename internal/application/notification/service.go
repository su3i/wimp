package notification

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/su3i/wimp/internal/application/alertmanager"
	"github.com/su3i/wimp/internal/application/recovery"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type wsMessage struct {
	Type    string                     `json:"type"`
	Payload *notification.Notification `json:"payload"`
}

// EmitAlert is the single chokepoint every alert-worthy event in the system should go
// through. It resolves the configured (or default) severity for the alert type, skips
// entirely if that resolves to Disabled, otherwise creates+broadcasts+delivers the
// notification and - for Sev-level alerts - calls into the (currently inert) recovery
// seam.
func EmitAlert(alertType notification.AlertType, projectID, machineID uint, instance, title, detail string, cfg *config.DatabaseConfig) {
	meta, ok := notification.AlertTypeRegistry[alertType]
	if !ok {
		log.Printf("notification: unknown alert type %q, dropping", alertType)
		return
	}

	level := SeverityFor(alertType)
	if level == notification.LevelDisabled {
		return
	}

	Emit(projectID, machineID, level, meta.Category, title, detail, cfg, instance)

	if level == notification.LevelSev {
		recovery.Trigger(recovery.IncidentContext{
			AlertType: alertType,
			ProjectID: projectID,
			MachineID: machineID,
			Title:     title,
			Detail:    detail,
		})
	}
}

// SeverityFor resolves an alert type's effective severity: a configured override from
// internal/config/alerts.go if set, else the registry's default.
func SeverityFor(alertType notification.AlertType) notification.Level {
	if override := severityOverride(alertType); override != "" {
		if lvl, ok := parseLevel(override); ok {
			return lvl
		}
		log.Printf("notification: invalid severity override %q for %s, using default", override, alertType)
	}
	if meta, ok := notification.AlertTypeRegistry[alertType]; ok {
		return meta.DefaultSeverity
	}
	return notification.LevelInfo
}

func parseLevel(s string) (notification.Level, bool) {
	switch notification.Level(strings.ToLower(strings.TrimSpace(s))) {
	case notification.LevelInfo:
		return notification.LevelInfo, true
	case notification.LevelWarning:
		return notification.LevelWarning, true
	case notification.LevelCritical:
		return notification.LevelCritical, true
	case notification.LevelSev:
		return notification.LevelSev, true
	case notification.LevelDisabled:
		return notification.LevelDisabled, true
	default:
		return "", false
	}
}

// severityOverride returns the configured severity string for an alert type, or "" if
// unset. One explicit case per alert type - deliberately not reflection-based, so
// every mapping is greppable and traceable.
func severityOverride(alertType notification.AlertType) string {
	a := config.Alerts()
	switch alertType {
	case notification.AlertMachineConnected:
		return a.SeverityMachineConnected
	case notification.AlertMachineDisconnected:
		return a.SeverityMachineDisconnected
	case notification.AlertMachineShutdown:
		return a.SeverityMachineShutdown
	case notification.AlertMachineRestarting:
		return a.SeverityMachineRestarting
	case notification.AlertAgentUpdated:
		return a.SeverityAgentUpdated
	case notification.AlertAppPoolStopped:
		return a.SeverityAppPoolStopped
	case notification.AlertAppPoolStarted:
		return a.SeverityAppPoolStarted
	case notification.AlertSiteStopped:
		return a.SeveritySiteStopped
	case notification.AlertSiteStarted:
		return a.SeveritySiteStarted
	case notification.AlertWindowsExporterDown:
		return a.SeverityWindowsExporterDown
	case notification.AlertWindowsExporterUp:
		return a.SeverityWindowsExporterUp
	case notification.AlertFluentBitDown:
		return a.SeverityFluentBitDown
	case notification.AlertFluentBitUp:
		return a.SeverityFluentBitUp
	case notification.AlertHealthCheckDown:
		return a.SeverityHealthCheckDown
	case notification.AlertHealthCheckUp:
		return a.SeverityHealthCheckUp
	case notification.AlertHighCPU:
		return a.SeverityHighCPU
	case notification.AlertHighCPURecovered:
		return a.SeverityHighCPURecovered
	case notification.AlertHighMemory:
		return a.SeverityHighMemory
	case notification.AlertHighMemoryRecovered:
		return a.SeverityHighMemoryRecovered
	case notification.AlertLowDisk:
		return a.SeverityLowDisk
	case notification.AlertLowDiskRecovered:
		return a.SeverityLowDiskRecovered
	default:
		return ""
	}
}

// receiverMinSeverity resolves the configured cutoff for external delivery, defaulting
// to Warning when unset.
func receiverMinSeverity() notification.Level {
	if lvl, ok := parseLevel(config.Alerts().ReceiverMinSeverity); ok {
		return lvl
	}
	return notification.LevelWarning
}

// deepLink builds a link back into the web frontend for an outbound alert, or "" if
// webUrl isn't configured. Machine-sourced alerts link straight to that machine's
// detail page; alerts with no specific machine (e.g. health checks) fall back to the
// Activity page.
func deepLink(webUrl string, machineID uint) string {
	if webUrl == "" {
		return ""
	}
	base := strings.TrimRight(webUrl, "/")
	if machineID == 0 {
		return base + "/activity"
	}
	return fmt.Sprintf("%s/machines/%d", base, machineID)
}

// Emit creates, persists, broadcasts, and (if it clears the receiver-min-severity
// cutoff) delivers a notification to Alertmanager. Most callers should use EmitAlert
// instead - this is the lower-level primitive it's built on.
func Emit(projectID, machineID uint, level notification.Level, category notification.Category, title, detail string, cfg *config.DatabaseConfig, instance string) {
	n := &notification.Notification{
		ProjectID: projectID,
		MachineID: machineID,
		Level:     level,
		Category:  category,
		Title:     title,
		Detail:    detail,
	}

	saved, err := database.NewNotificationRepository(cfg).Create(n)
	if err != nil {
		log.Printf("notification emit: failed to save: %v", err)
		return
	}

	msg, err := json.Marshal(wsMessage{Type: "notification", Payload: saved})
	if err != nil {
		return
	}
	hub.Clients().Broadcast(msg)

	if level.AtLeast(receiverMinSeverity()) {
		common := config.Common()
		generatorURL := deepLink(common.WebUrl, machineID)
		go func() {
			if common.AlertmanagerUrl == "" {
				log.Printf("alertmanager: skipped (ALERTMANAGERURL not set) - alert: %s", title)
				return
			}
			if err := alertmanager.Fire(common.AlertmanagerUrl, title, string(level), instance, string(category), detail, generatorURL); err != nil {
				log.Printf("alertmanager: failed to fire alert %q: %v", title, err)
				return
			}
			log.Printf("alertmanager: fired alert %q to %s", title, common.AlertmanagerUrl)
		}()
	}
}

func List(f notification.Filter, cfg *config.DatabaseConfig) ([]notification.Notification, int64, error) {
	return database.NewNotificationRepository(cfg).FindPaginated(f)
}

func UnreadCount(cfg *config.DatabaseConfig) (int64, error) {
	return database.NewNotificationRepository(cfg).UnreadCount()
}

func MarkRead(id uint, cfg *config.DatabaseConfig) error {
	return database.NewNotificationRepository(cfg).MarkRead(id)
}

func MarkAllRead(cfg *config.DatabaseConfig) error {
	return database.NewNotificationRepository(cfg).MarkAllRead()
}

func ActiveAlerts(cfg *config.DatabaseConfig) ([]notification.Notification, error) {
	return database.NewNotificationRepository(cfg).FindActiveAlerts()
}

func AlertHistory(hours int, cfg *config.DatabaseConfig) ([]notification.HourCount, error) {
	return database.NewNotificationRepository(cfg).CountByHour(hours)
}

func SevCount(since time.Time, projectKey string, cfg *config.DatabaseConfig) (int64, error) {
	return database.NewNotificationRepository(cfg).CountSevSince(since, projectKey)
}
