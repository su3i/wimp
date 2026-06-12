package notification

import (
	"time"

	"gorm.io/gorm"
)

type Level string
type Category string

const (
	LevelInfo     Level = "info"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"

	CategoryMachine Category = "machine"
	CategoryIIS     Category = "iis"
	CategoryAppPool Category = "apppool"
	CategoryService Category = "service"
)

type Notification struct {
	gorm.Model
	MachineID uint      `gorm:"not null;index"`
	Level     Level     `gorm:"type:text;not null"`
	Category  Category  `gorm:"type:text;not null"`
	Title     string    `gorm:"not null"`
	Detail    string
	ReadAt    *time.Time
}
