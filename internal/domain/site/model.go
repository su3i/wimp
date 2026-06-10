package site

import "gorm.io/gorm"

type Binding struct {
	Protocol string `json:"protocol"`
	IP       string `json:"ip"`
	Port     string `json:"port"`
	Hostname string `json:"hostname"`
}

type Site struct {
	gorm.Model

	MachineID    uint      `gorm:"not null;index"`
	Name         string    `gorm:"not null"`
	State        string    `gorm:"type:text;not null"`
	PhysicalPath string
	AppPoolName  string
	Bindings     []Binding `gorm:"serializer:json"`
}
