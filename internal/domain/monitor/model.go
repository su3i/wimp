package monitor

import "gorm.io/gorm"

type Monitor struct {
	gorm.Model
	ProjectID           uint   `gorm:"not null;index"`
	Name                string `gorm:"not null"`
	URL                 string `gorm:"not null"`
	IntervalSeconds     int    `gorm:"not null;default:60"`
	Enabled             bool   `gorm:"not null;default:true"`
	ConsecutiveFailures int    `gorm:"not null;default:0"`
	AlertFired          bool   `gorm:"not null;default:false"`
}
