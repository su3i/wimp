package notification

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/su3i/wimp/internal/application/telegram"
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
		text := fmt.Sprintf(
			"‼️ <b>%s</b>\n%s\n<i>%s</i>",
			title,
			detail,
			time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		)
		go func() {
			if err := telegram.Send(common.TelegramBotToken, common.TelegramChatID, text); err != nil {
				log.Printf("telegram alert: %v", err)
			}
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
