package metadata

import (
	"gorm.io/gorm"
)

type Metadata struct {
	gorm.Model

	BootstrapToken         string `gorm:"unique;not null"`
	Language         string `gorm:"not null"`
}