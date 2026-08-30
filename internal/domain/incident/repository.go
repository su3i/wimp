package incident

import (
	"time"

	"github.com/su3i/wimp/internal/domain/notification"
)

type Filter struct {
	ProjectID uint
	// ResolvedSince bounds how far back resolved incidents are returned. Open incidents
	// ignore it entirely - see the note on Repository.FindTimeline.
	ResolvedSince time.Time
	Page          int
	PerPage       int
}

type Counts struct {
	Open     int64
	Resolved int64
}

type Repository interface {
	Create(i *Incident) (*Incident, error)
	FindOneByID(id uint) (*Incident, error)
	// FindOpen locates the incident a closing alert belongs to. The four-part key is the
	// whole correlation mechanism: project, machine, condition and target.
	FindOpen(projectID, machineID uint, kind notification.IncidentKind, subject string) (*Incident, error)
	Resolve(id uint, resolvedAt any, title, detail string, notificationID uint) error
	// FindTimeline returns one ordered stream: everything still open, newest first, then
	// everything resolved, newest first.
	//
	// Open incidents are returned regardless of age. A window that hid them would hide the
	// outage that is happening right now the moment it passed its seventh day, which is
	// exactly when someone most needs to see it.
	FindTimeline(f Filter) ([]Incident, int64, error)
	CountByStatus(projectID uint) (Counts, error)
}
