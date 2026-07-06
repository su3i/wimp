package application

import "gorm.io/gorm"

type Application struct {
	gorm.Model

	ProjectID                  uint    `gorm:"not null;index"`
	Name                       string  `gorm:"not null"`
	HealthCheckURL             *string `gorm:"type:text"`
	HealthCheckIntervalSeconds int     `gorm:"not null;default:60"`
	ConsecutiveFailures        int     `gorm:"not null;default:0"`
	AlertFired                 bool    `gorm:"not null;default:false"`
}

type ApplicationAppPool struct {
	gorm.Model

	ApplicationID uint    `gorm:"not null;uniqueIndex:idx_application_app_pool"`
	AppPoolID     uint    `gorm:"not null;uniqueIndex:idx_application_app_pool"`
	LogPath       *string `gorm:"type:text"`
}
