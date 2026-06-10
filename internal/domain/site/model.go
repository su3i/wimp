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

	MachineID    uint      `gorm:"not null;uniqueIndex:idx_site_machine_name"`
	Name         string    `gorm:"not null;uniqueIndex:idx_site_machine_name"`
	State        string    `gorm:"type:text;not null"`
	PhysicalPath string
	AppPoolName  string
	Bindings     []Binding `gorm:"serializer:json"`
}
