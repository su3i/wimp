package incident

import "github.com/su3i/wimp/internal/domain/notification"

type Filter struct {
	ProjectID uint
	// Status empty means both open and resolved.
	Status  Status
	Page    int
	PerPage int
}

type Counts struct {
	Open     int64
	Resolved int64
}

type Repository interface {
	Create(i *Incident) (*Incident, error)
	// FindOpen locates the incident a closing alert belongs to. The four-part key is the
	// whole correlation mechanism: project, machine, condition and target.
	FindOpen(projectID, machineID uint, kind notification.IncidentKind, subject string) (*Incident, error)
	Resolve(id uint, resolvedAt any, title, detail string, notificationID uint) error
	FindPaginated(f Filter) ([]Incident, int64, error)
	CountByStatus(projectID uint) (Counts, error)
}
