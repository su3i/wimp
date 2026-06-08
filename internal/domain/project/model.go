package project

import (
	"gorm.io/gorm"
)

type Project struct {
	gorm.Model

	Name            string                   `gorm:"unique;not null"`
	Key             string                   `gorm:"unique;not null"`
	Status          ProjectStatus            `gorm:"type:text;not null"`
	BusinessDomain  string    				 `gorm:"not null"`
	CreatedBy       map[string]string 		 `gorm:"type:jsonb;serializer:json;default:'{}'"`
}
