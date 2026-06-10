package apppool

import "gorm.io/gorm"

type AppPool struct {
	gorm.Model

	MachineID      uint   `gorm:"not null;index"`
	Name           string `gorm:"not null"`
	State          string `gorm:"type:text;not null"`
	RuntimeVersion string
	PipelineMode   string
	StartMode      string
	IdentityType   string
}
