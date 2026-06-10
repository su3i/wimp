package application

import "gorm.io/gorm"

type Application struct {
	gorm.Model

	ProjectID uint   `gorm:"not null;index"`
	Name      string `gorm:"not null"`
}

type ApplicationAppPool struct {
	gorm.Model

	ApplicationID uint `gorm:"not null;uniqueIndex:idx_application_app_pool"`
	AppPoolID     uint `gorm:"not null;uniqueIndex:idx_application_app_pool"`
}
