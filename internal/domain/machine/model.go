package machine

import (
	"time"

	"gorm.io/gorm"
)

type Machine struct {
	gorm.Model

	ProjectID      uint          `gorm:"not null"`
	Hostname       string        `gorm:"default:null"`
	IPs            []string      `gorm:"serializer:json"`
	Status         MachineStatus `gorm:"type:text;not null"`
	Token          string        `gorm:"unique;not null"`
	TokenExpiresAt time.Time
	LastSeenAt     *time.Time
	AgentVersion   string `gorm:"default:null"`
	WindowsVersion string `gorm:"default:null"`
}
