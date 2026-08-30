// Package incident turns the stream of alerts into spans with a start and an end.
package incident

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/incident"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

// Alerts are emitted from their own goroutines (see the `go notificationService.EmitAlert`
// calls in ws.go), so two alerts for the same condition can land at once - a heartbeat
// reporting a pool stopped while the metrics checker reports the same host recovering, say.
// Read-then-write on the open incident is not atomic, so without this an interleaving can
// open two incidents for one condition, and only one of them will ever be closed.
//
// A single mutex rather than one per key: alert volume is low by nature (these are
// exceptional events, not a request path) and one lock is easier to reason about than a
// keyed map that has to be cleaned up. This assumes a single control-plane replica, which
// is what the Helm chart deploys; running several would need this moved into the database
// as a conditional insert.
var mu sync.Mutex

var (
	ErrNotFound = errors.New("incident not found")
	ErrNotOpen  = errors.New("incident is already resolved")
)

// Event is one alert offered to the incident tracker. Whether it opens anything, closes
// anything, or is ignored is decided entirely by notification.IncidentBindings.
type Event struct {
	AlertType      notification.AlertType
	ProjectID      uint
	MachineID      uint
	Instance       string
	Subject        string
	Level          notification.Level
	Category       notification.Category
	Title          string
	Detail         string
	NotificationID uint
	// Suppressed is true when the operator has this alert type set to "disabled", so no
	// notification was written. It still reaches us: see the note in Record.
	Suppressed bool
}

// Record folds one alert into the incident timeline.
func Record(e Event, cfg *config.DatabaseConfig) {
	binding, ok := notification.IncidentBindings[e.AlertType]
	if !ok {
		// Informational alert - an agent update, a planned restart. Nothing to track.
		return
	}

	mu.Lock()
	defer mu.Unlock()

	repo := database.NewIncidentRepository(cfg)

	open, err := repo.FindOpen(e.ProjectID, e.MachineID, binding.Kind, e.Subject)
	if err != nil {
		log.Printf("incident: lookup failed for %s/%s: %v", binding.Kind, e.Subject, err)
		return
	}

	switch binding.Role {
	case notification.RoleOpens:
		// Silencing an alert type means opting out of the condition altogether, so a
		// suppressed opening alert starts nothing.
		if e.Suppressed {
			return
		}
		if open != nil {
			// The condition is already being tracked. A metric that keeps breaching its
			// threshold re-fires, and each re-fire is the same incident, not a new one.
			return
		}
		created, err := repo.Create(&incident.Incident{
			ProjectID:            e.ProjectID,
			MachineID:            e.MachineID,
			Kind:                 binding.Kind,
			Subject:              e.Subject,
			Instance:             e.Instance,
			Level:                e.Level,
			Category:             e.Category,
			Status:               incident.StatusOpen,
			StartedAt:            time.Now(),
			OpenedTitle:          e.Title,
			OpenedDetail:         e.Detail,
			OpenedNotificationID: e.NotificationID,
		})
		if err != nil {
			log.Printf("incident: failed to open %s/%s: %v", binding.Kind, e.Subject, err)
			return
		}
		log.Printf("incident (%d) opened: %s on %s", created.ID, binding.Kind, e.Instance)

	case notification.RoleCloses:
		// Note the deliberate asymmetry with the open path above: a closing alert resolves
		// its incident even when suppressed. Silencing "high cpu recovered" should stop the
		// notification, not strand every CPU incident open forever.
		if open == nil {
			// A recovery with nothing to recover. Normal on startup, or after the control
			// plane restarts mid-incident.
			return
		}
		if err := repo.Resolve(open.ID, time.Now(), e.Title, e.Detail, e.NotificationID); err != nil {
			log.Printf("incident (%d): failed to resolve: %v", open.ID, err)
			return
		}
		log.Printf("incident (%d) resolved: %s on %s", open.ID, binding.Kind, e.Instance)
	}
}

// TimelineWindow is how far back resolved incidents are kept in the feed. Long enough to
// cover "what happened over the weekend", short enough that the list stays finite without
// the reader having to filter it. Incidents still open are exempt - see FindTimeline.
const TimelineWindow = 7 * 24 * time.Hour

// List returns the incident timeline: open first, newest first within each half.
func List(projectID uint, page, perPage int, cfg *config.DatabaseConfig) ([]incident.Incident, int64, error) {
	return database.NewIncidentRepository(cfg).FindTimeline(incident.Filter{
		ProjectID:     projectID,
		ResolvedSince: time.Now().Add(-TimelineWindow),
		Page:          page,
		PerPage:       perPage,
	})
}

// ResolveManually closes an incident by hand.
//
// This exists because not every condition reports its own recovery: an app pool removed
// from an application, a host decommissioned mid-outage, or a threshold alert whose
// recovery was silenced all leave an incident with nothing left to close it. The operator
// needs a way to say "this is over" without editing the database.
//
// Note it resolves the incident record only. It does not touch the underlying condition,
// and it does not reset the checker's in-memory breach state - so if the condition is
// genuinely still breached, the eventual real recovery alert will arrive and find the
// incident already closed, which is a no-op.
func ResolveManually(id, projectID uint, resolvedBy string, cfg *config.DatabaseConfig) error {
	mu.Lock()
	defer mu.Unlock()

	repo := database.NewIncidentRepository(cfg)

	found, err := repo.FindOneByID(id)
	if err != nil {
		return err
	}
	if found == nil || found.ProjectID != projectID {
		return ErrNotFound
	}
	if found.Status != incident.StatusOpen {
		return ErrNotOpen
	}

	detail := "Marked resolved manually"
	if resolvedBy != "" {
		detail = "Marked resolved by " + resolvedBy
	}
	return repo.Resolve(found.ID, time.Now(), "Resolved manually", detail, 0)
}

func Counts(projectID uint, cfg *config.DatabaseConfig) (incident.Counts, error) {
	return database.NewIncidentRepository(cfg).CountByStatus(projectID)
}
