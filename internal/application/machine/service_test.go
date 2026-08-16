package machine

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/su3i/wimp/internal/config"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	sqliteDB "github.com/su3i/wimp/internal/infrastructure/database/sqlite"
)

// ReconcileReassignment decides whether an existing host entry is retired, so a false
// positive silently takes a live machine out of its project's Prometheus targets. The
// cases below pin down when it fires and, more importantly, when it must not.

func setupDB(t *testing.T) *config.DatabaseConfig {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:reconcile?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&machineDomain.Machine{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("DELETE FROM machines").Error; err != nil {
		t.Fatalf("reset table: %v", err)
	}

	// The repository factory resolves through this package-level handle, and an empty
	// DatabaseConfig falls through to the sqlite branch.
	sqliteDB.DB = db
	return &config.DatabaseConfig{}
}

func insert(t *testing.T, m *machineDomain.Machine) *machineDomain.Machine {
	t.Helper()
	if m.Status == "" {
		m.Status = machineDomain.Online
	}
	m.Token = t.Name() + m.Hostname + string(m.Status) + m.MachineUID + time.Now().Format("150405.000000000")
	m.TokenExpiresAt = time.Now().Add(time.Hour)
	if err := sqliteDB.DB.Create(m).Error; err != nil {
		t.Fatalf("insert machine: %v", err)
	}
	return m
}

func statusOf(t *testing.T, id uint) machineDomain.MachineStatus {
	t.Helper()
	var m machineDomain.Machine
	if err := sqliteDB.DB.First(&m, id).Error; err != nil {
		t.Fatalf("reload machine %d: %v", id, err)
	}
	return m.Status
}

func neverOnline(uint) bool { return false }

func TestReconcileRetiresPreviousProjectEntry(t *testing.T) {
	cfg := setupDB(t)

	old := insert(t, &machineDomain.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a", Status: machineDomain.Offline})
	fresh := insert(t, &machineDomain.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	retired := ReconcileReassignment(fresh, neverOnline, cfg)

	if len(retired) != 1 || retired[0].ID != old.ID {
		t.Fatalf("expected the old project's entry to be retired, got %+v", retired)
	}
	if got := statusOf(t, old.ID); got != machineDomain.Reassigned {
		t.Fatalf("old entry status = %q, want %q", got, machineDomain.Reassigned)
	}
	if got := statusOf(t, fresh.ID); got != machineDomain.Online {
		t.Fatalf("the newly registered entry must be untouched, got %q", got)
	}
}

func TestReconcileLeavesLiveSameHostnameMachineAlone(t *testing.T) {
	cfg := setupDB(t)

	// Two genuinely different boxes that happen to share a hostname across environments,
	// where the older row predates UID reporting so only the hostname can be compared.
	live := insert(t, &machineDomain.Machine{ProjectID: 1, Hostname: "web01"})
	fresh := insert(t, &machineDomain.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	alwaysOnline := func(id uint) bool { return id == live.ID }

	retired := ReconcileReassignment(fresh, alwaysOnline, cfg)

	if len(retired) != 0 {
		t.Fatalf("a currently connected machine must never be retired on a hostname match, got %+v", retired)
	}
	if got := statusOf(t, live.ID); got != machineDomain.Online {
		t.Fatalf("live machine status = %q, want it left online", got)
	}
}

func TestReconcileRetiresOnUIDEvenWhenPredecessorStillConnected(t *testing.T) {
	cfg := setupDB(t)

	// A UID match is the same physical box by definition, so a lingering connection on
	// the old row (agent not yet noticed it was replaced) must not block the retirement.
	old := insert(t, &machineDomain.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a"})
	fresh := insert(t, &machineDomain.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	alwaysOnline := func(uint) bool { return true }

	retired := ReconcileReassignment(fresh, alwaysOnline, cfg)

	if len(retired) != 1 || retired[0].ID != old.ID {
		t.Fatalf("expected the UID match to be retired regardless of connection state, got %+v", retired)
	}
	if got := statusOf(t, old.ID); got != machineDomain.Reassigned {
		t.Fatalf("old entry status = %q, want %q", got, machineDomain.Reassigned)
	}
}

func TestReconcileIgnoresUnrelatedMachines(t *testing.T) {
	cfg := setupDB(t)

	other := insert(t, &machineDomain.Machine{ProjectID: 1, Hostname: "db01", MachineUID: "uid-b"})
	fresh := insert(t, &machineDomain.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	if retired := ReconcileReassignment(fresh, neverOnline, cfg); len(retired) != 0 {
		t.Fatalf("expected no matches, got %+v", retired)
	}
	if got := statusOf(t, other.ID); got != machineDomain.Online {
		t.Fatalf("unrelated machine status = %q, want online", got)
	}
}

func TestReconcileIsNoOpForAnAgentSimplyReconnecting(t *testing.T) {
	cfg := setupDB(t)

	// The overwhelmingly common path: one machine, one row, agent reconnects after a
	// restart. Nothing should be retired, least of all itself.
	m := insert(t, &machineDomain.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a"})

	if retired := ReconcileReassignment(m, neverOnline, cfg); len(retired) != 0 {
		t.Fatalf("a plain reconnect must retire nothing, got %+v", retired)
	}
	if got := statusOf(t, m.ID); got != machineDomain.Online {
		t.Fatalf("status = %q, want online", got)
	}
}

func TestReconcileSkipsMachineWithNoIdentityYet(t *testing.T) {
	cfg := setupDB(t)

	insert(t, &machineDomain.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a"})
	// A row created by the API but never registered has neither hostname nor UID.
	pending := insert(t, &machineDomain.Machine{ProjectID: 2, Status: machineDomain.Pending})

	if retired := ReconcileReassignment(pending, neverOnline, cfg); len(retired) != 0 {
		t.Fatalf("a machine with no identity must match nothing, got %+v", retired)
	}
}
