package organization

import (
	"gorm.io/gorm"
)

type Organization struct {
	gorm.Model

	Name         string `gorm:"unique;not null"`
	Key         string `gorm:"unique;not null"`
	Scope		 OrgScope `gorm:"type:text;not null"`
}