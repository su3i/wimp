// Package incident models the lifecycle of a fault condition, assembled from the alerts
// that already flow through internal/application/notification.
//
// The insight this is built on: every fault-shaped alert in the system already exists as a
// pair - something breaks, something recovers. high_cpu has high_cpu_recovered, site_stopped
// has site_started. Individually those are two unrelated lines in an activity feed, and the
// operator is left to eyeball which recovery belongs to which failure. An Incident is the
// span between them: one row that opens when the first alert fires and closes when its
// partner does, so "how long was CPU pinned on WEB01" is a stored fact rather than a manual
// diff of two timestamps.
package incident

import (
	"time"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/notification"
)

type Status string

const (
	StatusOpen     Status = "open"
	StatusResolved Status = "resolved"
)

type Incident struct {
	gorm.Model

	ProjectID uint `gorm:"not null;index:idx_incident_project_status,priority:1"`
	// MachineID is 0 for conditions that are not about a host - a failing application
	// health check belongs to the project, not to any one machine.
	MachineID uint `gorm:"not null"`

	// Kind identifies the condition, shared by the alert that opens the incident and the
	// one that closes it (see notification.IncidentBindings). It is what makes a recovery
	// findable from a failure.
	Kind notification.IncidentKind `gorm:"type:text;not null"`
	// Subject narrows the condition to one target within the machine - an app pool name, a
	// site name, an application. Empty for conditions that are about the machine itself.
	// Without it, two app pools failing on the same host would share one incident, and the
	// first recovery would wrongly close both.
	Subject string `gorm:"type:text;not null;default:''"`
	// Instance is the display name of whatever the incident is about (hostname, or the
	// application name for health checks). Denormalized so the timeline needs no joins.
	Instance string `gorm:"type:text;not null;default:''"`

	Level    notification.Level    `gorm:"type:text;not null"`
	Category notification.Category `gorm:"type:text;not null"`

	Status Status `gorm:"type:text;not null;index:idx_incident_project_status,priority:2"`

	StartedAt  time.Time  `gorm:"not null"`
	ResolvedAt *time.Time

	// Both ends of the span are denormalized rather than joined back to notifications.
	// Notifications are user-facing records that can be read, filtered and eventually
	// pruned; an incident's own history should not develop holes when that happens.
	OpenedTitle    string `gorm:"not null;default:''"`
	OpenedDetail   string `gorm:"type:text;not null;default:''"`
	ResolvedTitle  string `gorm:"not null;default:''"`
	ResolvedDetail string `gorm:"type:text;not null;default:''"`

	OpenedNotificationID   uint
	ResolvedNotificationID *uint
}

// Duration returns how long the incident lasted, measured to now while it is still open.
func (i Incident) Duration(now time.Time) time.Duration {
	if i.ResolvedAt != nil {
		return i.ResolvedAt.Sub(i.StartedAt)
	}
	return now.Sub(i.StartedAt)
}
