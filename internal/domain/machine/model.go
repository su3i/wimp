package machine

import (
	"time"

	"gorm.io/gorm"
)

type Machine struct {
	gorm.Model

	ProjectID      uint          `gorm:"not null"`
	Hostname       string        // set by agent on first connect
	IP             string        // set from websocket remote address on first connect
	Status         MachineStatus `gorm:"type:text;not null"`
	Token          string        `gorm:"unique;not null"`
	TokenExpiresAt time.Time
	LastSeenAt     *time.Time
}
