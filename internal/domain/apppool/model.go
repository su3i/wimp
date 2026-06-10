package apppool

import "gorm.io/gorm"

type AppPool struct {
	gorm.Model

	MachineID      uint   `gorm:"not null;uniqueIndex:idx_apppool_machine_name"`
	Name           string `gorm:"not null;uniqueIndex:idx_apppool_machine_name"`
	State          string `gorm:"type:text;not null"`
	RuntimeVersion string
	PipelineMode   string
	StartMode      string
	IdentityType   string
}
