package account

import (
	"gorm.io/gorm"
)

type Account struct {
	gorm.Model

	Name         string `gorm:"unique;not null"`
	Email         string `gorm:"unique;not null"`
	PasswordEnc		string
	Role 		AccountRole `gorm:"type:text;not null"`
	InternalRoles map[string]string `gorm:"type:jsonb;serializer:json;default:'{}'"`

	MFAEnabled    bool
	MFASecret     string `gorm:"unique;not null"`
}