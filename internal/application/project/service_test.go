package project

import (
	"errors"
	"testing"

	"github.com/su3i/wimp/internal/config"
	projectDomain "github.com/su3i/wimp/internal/domain/project"
)

// The default project is where the UI sends a user whose current project was just deleted.
// If it could itself be deleted, that fallback would point at nothing.
func TestDefaultProjectCannotBeDeleted(t *testing.T) {
	// No database is touched: the guard runs before any lookup, which is what makes this
	// safe to assert without one.
	err := DeleteProject(projectDomain.DefaultKey, &config.DatabaseConfig{})
	if !errors.Is(err, ErrCannotDeleteDefault) {
		t.Fatalf("error = %v, want ErrCannotDeleteDefault", err)
	}
}
