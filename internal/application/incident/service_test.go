package incident

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/su3i/wimp/internal/config"
	incidentDomain "github.com/su3i/wimp/internal/domain/incident"
	"github.com/su3i/wimp/internal/domain/notification"
	sqliteDB "github.com/su3i/wimp/internal/infrastructure/database/sqlite"
)

// Incident correlation is the whole feature: an alert has to find the failure it recovers
// from, and only that one. These pin down the pairing rules, since getting them wrong
// either strands incidents open forever or closes the wrong one.

func setupDB(t *testing.T) *config.DatabaseConfig {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:incidents?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&incidentDomain.Incident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("DELETE FROM incidents").Error; err != nil {
		t.Fatalf("reset table: %v", err)
	}

	sqliteDB.DB = db
	return &config.DatabaseConfig{}
}

func alert(t notification.AlertType, machineID uint, subject string) Event {
	return Event{
		AlertType: t,
		ProjectID: 1,
		MachineID: machineID,
		Instance:  "web01",
		Subject:   subject,
		Level:     notification.LevelCritical,
		Category:  notification.CategoryAppPool,
		Title:     string(t),
	}
}

func list(t *testing.T, cfg *config.DatabaseConfig) []incidentDomain.Incident {
	t.Helper()
	out, _, err := List(1, "", 1, 100, cfg)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return out
}

func TestOpenThenCloseFormsOneResolvedIncident(t *testing.T) {
	cfg := setupDB(t)

	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	Record(alert(notification.AlertHighCPURecovered, 7, ""), cfg)

	got := list(t, cfg)
	if len(got) != 1 {
		t.Fatalf("expected one incident, got %d", len(got))
	}
	if got[0].Status != incidentDomain.StatusResolved {
		t.Fatalf("status = %q, want resolved", got[0].Status)
	}
	if got[0].ResolvedAt == nil {
		t.Fatal("resolved incident has no ResolvedAt")
	}
	if got[0].Kind != notification.IncidentHighCPU {
		t.Fatalf("kind = %q, want %q", got[0].Kind, notification.IncidentHighCPU)
	}
}

func TestRefiringDoesNotOpenASecondIncident(t *testing.T) {
	cfg := setupDB(t)

	// A metric that stays over its threshold re-alerts; that is the same incident.
	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	Record(alert(notification.AlertHighCPU, 7, ""), cfg)

	if got := list(t, cfg); len(got) != 1 {
		t.Fatalf("expected one incident, got %d", len(got))
	}
}

func TestSubjectKeepsSiblingsOnOneHostApart(t *testing.T) {
	cfg := setupDB(t)

	// Two pools failing on the same machine, then only one recovering. Without the subject
	// in the key, that recovery would close both.
	Record(alert(notification.AlertAppPoolStopped, 7, "api-pool"), cfg)
	Record(alert(notification.AlertAppPoolStopped, 7, "web-pool"), cfg)
	Record(alert(notification.AlertAppPoolStarted, 7, "api-pool"), cfg)

	got := list(t, cfg)
	if len(got) != 2 {
		t.Fatalf("expected two incidents, got %d", len(got))
	}

	bySubject := map[string]incidentDomain.Status{}
	for _, i := range got {
		bySubject[i.Subject] = i.Status
	}
	if bySubject["api-pool"] != incidentDomain.StatusResolved {
		t.Fatalf("api-pool = %q, want resolved", bySubject["api-pool"])
	}
	if bySubject["web-pool"] != incidentDomain.StatusOpen {
		t.Fatalf("web-pool = %q, want it left open", bySubject["web-pool"])
	}
}

func TestSameConditionOnDifferentHostsStaysSeparate(t *testing.T) {
	cfg := setupDB(t)

	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	Record(alert(notification.AlertHighCPU, 8, ""), cfg)
	Record(alert(notification.AlertHighCPURecovered, 7, ""), cfg)

	got := list(t, cfg)
	if len(got) != 2 {
		t.Fatalf("expected one incident per host, got %d", len(got))
	}
	for _, i := range got {
		want := incidentDomain.StatusOpen
		if i.MachineID == 7 {
			want = incidentDomain.StatusResolved
		}
		if i.Status != want {
			t.Fatalf("machine %d status = %q, want %q", i.MachineID, i.Status, want)
		}
	}
}

func TestRecoveryWithNoOpenIncidentIsIgnored(t *testing.T) {
	cfg := setupDB(t)

	// Normal after a control plane restart: the recovery arrives with nothing to close.
	Record(alert(notification.AlertHighCPURecovered, 7, ""), cfg)

	if got := list(t, cfg); len(got) != 0 {
		t.Fatalf("a recovery alone must not create anything, got %d", len(got))
	}
}

func TestConditionsOnOneHostDoNotCollide(t *testing.T) {
	cfg := setupDB(t)

	// High CPU and low disk on the same host are two separate incidents, and a CPU
	// recovery must not close the disk one.
	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	Record(alert(notification.AlertLowDisk, 7, ""), cfg)
	Record(alert(notification.AlertHighCPURecovered, 7, ""), cfg)

	got := list(t, cfg)
	if len(got) != 2 {
		t.Fatalf("expected two incidents, got %d", len(got))
	}
	byKind := map[notification.IncidentKind]incidentDomain.Status{}
	for _, i := range got {
		byKind[i.Kind] = i.Status
	}
	if byKind[notification.IncidentHighCPU] != incidentDomain.StatusResolved {
		t.Fatal("cpu incident should be resolved")
	}
	if byKind[notification.IncidentLowDisk] != incidentDomain.StatusOpen {
		t.Fatal("disk incident should still be open")
	}
}

func TestInformationalAlertsTrackNothing(t *testing.T) {
	cfg := setupDB(t)

	// A planned restart is not a fault, and neither is an agent update. Only an
	// unexplained disconnect opens an incident.
	Record(alert(notification.AlertMachineRestarting, 7, ""), cfg)
	Record(alert(notification.AlertAgentUpdated, 7, ""), cfg)
	Record(alert(notification.AlertMachineShutdown, 7, ""), cfg)

	if got := list(t, cfg); len(got) != 0 {
		t.Fatalf("informational alerts must not open incidents, got %d", len(got))
	}
}

func TestSuppressedOpenIsSkippedButSuppressedCloseStillResolves(t *testing.T) {
	cfg := setupDB(t)

	// Silencing an alert type opts out of the condition, so nothing is opened.
	suppressedOpen := alert(notification.AlertHighCPU, 7, "")
	suppressedOpen.Suppressed = true
	Record(suppressedOpen, cfg)
	if got := list(t, cfg); len(got) != 0 {
		t.Fatalf("a suppressed opening alert must not open an incident, got %d", len(got))
	}

	// But silencing the recovery must not strand an incident that did open, or it would
	// stay open forever with no way to close it.
	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	suppressedClose := alert(notification.AlertHighCPURecovered, 7, "")
	suppressedClose.Suppressed = true
	Record(suppressedClose, cfg)

	got := list(t, cfg)
	if len(got) != 1 || got[0].Status != incidentDomain.StatusResolved {
		t.Fatalf("expected one resolved incident, got %+v", got)
	}
}

func TestCountsSplitByStatus(t *testing.T) {
	cfg := setupDB(t)

	Record(alert(notification.AlertHighCPU, 7, ""), cfg)
	Record(alert(notification.AlertHighCPU, 8, ""), cfg)
	Record(alert(notification.AlertHighCPURecovered, 8, ""), cfg)

	counts, err := Counts(1, cfg)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.Open != 1 || counts.Resolved != 1 {
		t.Fatalf("counts = %+v, want 1 open / 1 resolved", counts)
	}
}
