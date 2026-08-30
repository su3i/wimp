package notification

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/su3i/wimp/internal/application/alertmanager"
	incidentService "github.com/su3i/wimp/internal/application/incident"
	"github.com/su3i/wimp/internal/application/recovery"
	settingService "github.com/su3i/wimp/internal/application/setting"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type wsMessage struct {
	Type    string                     `json:"type"`
	Payload *notification.Notification `json:"payload"`
}

// AlertTitle formats an alert title as "<INSTANCE> // <event>", with instance always
// uppercased so titles scan consistently regardless of a machine's actual hostname casing.
func AlertTitle(instance, event string) string {
	return strings.ToUpper(instance) + " // " + event
}

// EmitAlert is the single chokepoint every alert-worthy event in the system should go
// through. It resolves the configured (or default) severity for the alert type, creates,
// broadcasts and delivers the notification unless that severity is Disabled, folds the
// event into the incident timeline, and - for Sev-level alerts - calls into the
// (currently inert) recovery seam.
//
// `subject` names the specific thing the alert is about within its machine - an app pool
// name, a site name, an application. It is what lets a recovery be paired with the failure
// it recovers from: two app pools failing on one host produce two incidents, and each
// recovery closes the right one. Pass "" when the condition is about the machine as a
// whole (high CPU, host offline), where the machine id is already the whole identity.
func EmitAlert(alertType notification.AlertType, projectID, machineID uint, instance, subject, title, detail string, cfg *config.DatabaseConfig) {
	emitAlert(alertType, projectID, machineID, instance, subject, title, detail, cfg, false)
}

// EmitRepeatAlert behaves like EmitAlert but marks the notification as a reminder for
// an already-firing alert (Notification.IsRepeat) rather than a new incident, so it
// doesn't re-trigger recovery and doesn't get double-counted by incident-count metrics
// like the dashboard's Sev Events card. Used by internal/application/metrics/checker.go
// for the 15-minute sustained-Sev-breach reminder.
func EmitRepeatAlert(alertType notification.AlertType, projectID, machineID uint, instance, subject, title, detail string, cfg *config.DatabaseConfig) {
	emitAlert(alertType, projectID, machineID, instance, subject, title, detail, cfg, true)
}

func emitAlert(alertType notification.AlertType, projectID, machineID uint, instance, subject, title, detail string, cfg *config.DatabaseConfig, isRepeat bool) {
	meta, ok := notification.AlertTypeRegistry[alertType]
	if !ok {
		log.Printf("notification: unknown alert type %q, dropping", alertType)
		return
	}

	level := SeverityFor(alertType)
	suppressed := level == notification.LevelDisabled

	var notificationID uint
	if !suppressed {
		if saved := Emit(projectID, machineID, level, meta.Category, title, detail, cfg, instance, isRepeat); saved != nil {
			notificationID = saved.ID
		}
	}

	// Repeats are reminders about a condition already being tracked, so they must not open
	// a second incident or close the one that is running.
	if !isRepeat {
		incidentService.Record(incidentService.Event{
			AlertType:      alertType,
			ProjectID:      projectID,
			MachineID:      machineID,
			Instance:       instance,
			Subject:        subject,
			Level:          level,
			Category:       meta.Category,
			Title:          title,
			Detail:         detail,
			NotificationID: notificationID,
			Suppressed:     suppressed,
		}, cfg)
	}

	if suppressed {
		return
	}

	if level == notification.LevelSev && !isRepeat {
		recovery.Trigger(recovery.IncidentContext{
			AlertType: alertType,
			ProjectID: projectID,
			MachineID: machineID,
			Title:     title,
			Detail:    detail,
		})
	}
}

// SeverityFor resolves an alert type's effective severity. Operator overrides win, then
// the deployment's environment, then the registry default - see internal/application/setting.
func SeverityFor(alertType notification.AlertType) notification.Level {
	level, _ := settingService.SeverityFor(alertType)
	return level
}

// receiverMinSeverity resolves the cutoff for external delivery.
func receiverMinSeverity() notification.Level {
	level, _ := settingService.ReceiverMinSeverity()
	return level
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
	return fmt.Sprintf("%s/hosts/%d", base, machineID)
}

// Emit creates, persists, broadcasts, and (if it clears the receiver-min-severity
// cutoff) delivers a notification to Alertmanager. Most callers should use EmitAlert
// instead - this is the lower-level primitive it's built on.
func Emit(projectID, machineID uint, level notification.Level, category notification.Category, title, detail string, cfg *config.DatabaseConfig, instance string, isRepeat bool) *notification.Notification {
	n := &notification.Notification{
		ProjectID: projectID,
		MachineID: machineID,
		Level:     level,
		Category:  category,
		Title:     title,
		Detail:    detail,
		IsRepeat:  isRepeat,
	}

	saved, err := database.NewNotificationRepository(cfg).Create(n)
	if err != nil {
		log.Printf("notification emit: failed to save: %v", err)
		return nil
	}

	msg, err := json.Marshal(wsMessage{Type: "notification", Payload: saved})
	if err != nil {
		return saved
	}
	hub.Clients().Broadcast(msg)

	if level.AtLeast(receiverMinSeverity()) {
		common := config.Common()
		generatorURL := deepLink(common.WebUrl, machineID)
		go func() {
			if !settingService.AlertingEnabled() {
				log.Printf("alertmanager: skipped (outbound alerting is switched off) - alert: %s", title)
				return
			}
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

	return saved
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
