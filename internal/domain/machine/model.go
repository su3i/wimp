package machine

import (
	"time"

	"gorm.io/gorm"
)

type Machine struct {
	gorm.Model

	ProjectID      uint          `gorm:"not null"`
	OsType         OsType        `gorm:"type:text;not null;default:'windows'"`
	Hostname       string        `gorm:"default:null"`
	IPs            []string      `gorm:"serializer:json"`
	Status         MachineStatus `gorm:"type:text;not null"`
	Token          string        `gorm:"unique;not null"`
	TokenExpiresAt time.Time
	LastSeenAt     *time.Time
	AgentVersion   string `gorm:"default:null"`
	WindowsVersion string `gorm:"default:null"`

	// MachineUID is the agent's stable identifier for the physical box (Windows
	// MachineGuid), reported on every register. It is what makes re-bootstrapping a host
	// into another project detectable: the token and row change, this does not. Empty for
	// agents older than the build that started reporting it, and deliberately not unique
	// at the DB level - two rows legitimately share a UID for the moment between a
	// re-bootstrap and the old row being marked Reassigned.
	MachineUID string `gorm:"index;default:null"`
	// SupersededByID points at the machine row that took this one's place when it was
	// reassigned, so the old project can say where the host went rather than just showing
	// it as offline.
	SupersededByID *uint      `gorm:"default:null"`
	ReassignedAt   *time.Time `gorm:"default:null"`

	// Cluster is the failover cluster this host is a node of, as last reported by its agent.
	// Nil for hosts that are not cluster nodes.
	Cluster *Cluster `gorm:"serializer:json"`
}

// Cluster mirrors protocol.ClusterInfo, the same way site.Binding mirrors the wire type, so
// the stored shape does not change just because the protocol does.
type Cluster struct {
	Name               string              `json:"name"`
	Nodes              []ClusterNode       `json:"nodes"`
	AvailabilityGroups []AvailabilityGroup `json:"availability_groups"`
}

type ClusterNode struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type AvailabilityGroup struct {
	Name        string `json:"name"`
	PrimaryNode string `json:"primary_node"`
	State       string `json:"state"`
}
