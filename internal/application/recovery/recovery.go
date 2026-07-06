// Package recovery is the seam a future incident-recovery workflow engine hangs off
// of. Recovery is a multi-step workflow (try X, check, try Y if that didn't resolve
// it, ...) - deciding what that workflow looks like is explicitly out of scope for now.
// This package only guarantees that every Sev-level alert reliably reaches exactly one
// obvious place.
package recovery

import (
	"log"

	"github.com/su3i/wimp/internal/domain/notification"
)

type IncidentContext struct {
	AlertType notification.AlertType
	ProjectID uint
	MachineID uint
	Title     string
	Detail    string
}

// Trigger is called whenever an alert is emitted at Sev severity. It is intentionally
// a no-op today - a future sprint builds the actual recovery workflow engine behind
// this call site.
func Trigger(ctx IncidentContext) {
	log.Printf("recovery: incident trigger reached for %s (project %d, machine %d) - no workflow wired yet", ctx.AlertType, ctx.ProjectID, ctx.MachineID)
}
