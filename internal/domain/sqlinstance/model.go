package sqlinstance

import "gorm.io/gorm"

// SqlInstance is one SQL Server instance installed on a machine, kept in sync from agent
// discovery the same way AppPool is. A failover cluster instance has one row per node it is
// installed on, each carrying the same cluster-wide state and owner.
type SqlInstance struct {
	gorm.Model

	MachineID        uint   `gorm:"not null;uniqueIndex:idx_sqlinstance_machine_name"`
	Name             string `gorm:"not null;uniqueIndex:idx_sqlinstance_machine_name"`
	ServiceName      string
	AgentServiceName string
	Version          string
	Edition          string
	Port             string
	State            string `gorm:"type:text;not null"`
	AgentState       string `gorm:"type:text"`

	Clustered         bool `gorm:"not null;default:false"`
	VirtualServerName string
	OwnerNode         string

	// Monitored is set on the row of the node querying this instance's DMVs; MonitorError is
	// that node's last failure, empty when its last poll succeeded.
	Monitored    bool `gorm:"not null;default:false"`
	MonitorError string
}
