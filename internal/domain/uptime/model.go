package uptime

import (
	"time"

	"gorm.io/gorm"
)

type EventType string

const (
	EventOnline  EventType = "online"
	EventOffline EventType = "offline"
)

type Event struct {
	gorm.Model
	MachineID  uint      `gorm:"not null;index"`
	Event      EventType `gorm:"type:text;not null"`
	OccurredAt time.Time `gorm:"not null;index"`
}
