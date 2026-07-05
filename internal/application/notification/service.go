package notification

import (
	"encoding/json"
	"log"
	"time"

	"github.com/su3i/wimp/internal/application/alertmanager"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type wsMessage struct {
	Type    string               `json:"type"`
	Payload *notification.Notification `json:"payload"`
}

func Emit(machineID uint, level notification.Level, category notification.Category, title, detail string, cfg *config.DatabaseConfig) {
	n := &notification.Notification{
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

	if level == notification.LevelCritical {
		common := config.Common()
		go func() {
			if common.AlertmanagerUrl == "" {
				log.Printf("alertmanager: skipped (ALERTMANAGERURL not set) - alert: %s", title)
				return
			}
			if err := alertmanager.Fire(common.AlertmanagerUrl, title, "critical", "", detail); err != nil {
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

func CriticalCount(since time.Time, projectKey string, cfg *config.DatabaseConfig) (int64, error) {
	return database.NewNotificationRepository(cfg).CountCriticalSince(since, projectKey)
}
