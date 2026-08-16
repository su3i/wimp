package repositories

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/su3i/wimp/internal/domain/machine"
)

// These cover the queries behind host reassignment - re-bootstrapping a machine that WIMP
// already knows, usually to move it into another project. The matching rules are easy to
// get subtly wrong in a way that only shows up against a real database (the COALESCE on a
// nullable column, the typed-slice NOT IN), and getting them wrong either fails to retire
// a stale row or wrongly retires a live unrelated one.

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&machine.Machine{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// cache=shared keeps one database alive for the whole process, so each test has to
	// start from a clean table rather than inheriting the previous test's rows.
	if err := db.Exec("DELETE FROM machines").Error; err != nil {
		t.Fatalf("reset table: %v", err)
	}
	return db
}

func seed(t *testing.T, db *gorm.DB, m machine.Machine) machine.Machine {
	t.Helper()
	if m.Token == "" {
		m.Token = t.Name() + "-" + m.Hostname + "-" + string(m.Status) + m.MachineUID
	}
	if m.Status == "" {
		m.Status = machine.Online
	}
	m.TokenExpiresAt = time.Now().Add(time.Hour)
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed machine: %v", err)
	}
	return m
}

func ids(machines *[]machine.Machine) []uint {
	out := []uint{}
	for _, m := range *machines {
		out = append(out, m.ID)
	}
	return out
}

func TestFindPredecessorsMatchesOnMachineUID(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	old := seed(t, db, machine.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a", Status: machine.Offline})
	fresh := seed(t, db, machine.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})
	// Same hostname, different box: must not be swept up.
	seed(t, db, machine.Machine{ProjectID: 3, Hostname: "web01", MachineUID: "uid-b"})

	got, err := repo.FindPredecessors(fresh.ID, "uid-a", "web01")
	if err != nil {
		t.Fatalf("FindPredecessors: %v", err)
	}
	if len(*got) != 1 || (*got)[0].ID != old.ID {
		t.Fatalf("expected only the uid-a predecessor %d, got %v", old.ID, ids(got))
	}
}

func TestFindPredecessorsFallsBackToHostnameForUnknownUID(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	// Row predating UID reporting - the agent that created it never sent one.
	legacy := seed(t, db, machine.Machine{ProjectID: 1, Hostname: "web01", Status: machine.Offline})
	fresh := seed(t, db, machine.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	got, err := repo.FindPredecessors(fresh.ID, "uid-a", "web01")
	if err != nil {
		t.Fatalf("FindPredecessors: %v", err)
	}
	if len(*got) != 1 || (*got)[0].ID != legacy.ID {
		t.Fatalf("expected the legacy row %d to match on hostname, got %v", legacy.ID, ids(got))
	}
}

func TestFindPredecessorsSkipsTerminalRows(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	seed(t, db, machine.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a", Status: machine.Reassigned})
	seed(t, db, machine.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a", Status: machine.Deleting})
	fresh := seed(t, db, machine.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	got, err := repo.FindPredecessors(fresh.ID, "uid-a", "web01")
	if err != nil {
		t.Fatalf("FindPredecessors: %v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected already-retired rows to be skipped, got %v", ids(got))
	}
}

func TestFindPredecessorsExcludesItself(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	self := seed(t, db, machine.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a"})

	got, err := repo.FindPredecessors(self.ID, "uid-a", "web01")
	if err != nil {
		t.Fatalf("FindPredecessors: %v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("a reconnecting agent must not retire its own row, got %v", ids(got))
	}
}

func TestFindPredecessorsWithNoIdentityMatchesNothing(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	seed(t, db, machine.Machine{ProjectID: 1, Hostname: "", MachineUID: ""})
	fresh := seed(t, db, machine.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	// A machine that has never registered has neither hostname nor UID; it must never be
	// treated as a predecessor of anything.
	got, err := repo.FindPredecessors(fresh.ID, "", "")
	if err != nil {
		t.Fatalf("FindPredecessors: %v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("expected no matches without an identity, got %v", ids(got))
	}
}

func TestMarkReassignedSetsTerminalState(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	old := seed(t, db, machine.Machine{ProjectID: 1, Hostname: "web01", MachineUID: "uid-a", Status: machine.Online})
	fresh := seed(t, db, machine.Machine{ProjectID: 2, Hostname: "web01", MachineUID: "uid-a"})

	if err := repo.MarkReassigned(old.ID, fresh.ID); err != nil {
		t.Fatalf("MarkReassigned: %v", err)
	}

	got, err := repo.FindOneByID(old.ID)
	if err != nil || got == nil {
		t.Fatalf("FindOneByID: %v", err)
	}
	if got.Status != machine.Reassigned {
		t.Fatalf("status = %q, want %q", got.Status, machine.Reassigned)
	}
	if got.SupersededByID == nil || *got.SupersededByID != fresh.ID {
		t.Fatalf("SupersededByID = %v, want %d", got.SupersededByID, fresh.ID)
	}
	if got.ReassignedAt == nil {
		t.Fatal("ReassignedAt was not set")
	}
	if !got.Status.Terminal() {
		t.Fatal("reassigned must count as terminal, or the row stays a Prometheus scrape target")
	}
}

func TestSetIdentityPreservesKnownValuesWhenAgentReportsNothing(t *testing.T) {
	db := newTestDB(t)
	repo := NewMachineRepository(db)

	m := seed(t, db, machine.Machine{
		ProjectID:      1,
		Hostname:       "web01",
		MachineUID:     "uid-a",
		WindowsVersion: "Windows Server 2019",
	})

	// An agent that cannot read the registry reports empty strings; those must not wipe
	// values an earlier registration established.
	if err := repo.SetIdentity(m.ID, "web01", []string{"10.0.0.4"}, "1.0.4", "", ""); err != nil {
		t.Fatalf("SetIdentity: %v", err)
	}

	got, err := repo.FindOneByID(m.ID)
	if err != nil || got == nil {
		t.Fatalf("FindOneByID: %v", err)
	}
	if got.MachineUID != "uid-a" {
		t.Fatalf("MachineUID = %q, want it preserved as uid-a", got.MachineUID)
	}
	if got.WindowsVersion != "Windows Server 2019" {
		t.Fatalf("WindowsVersion = %q, want it preserved", got.WindowsVersion)
	}
	if got.AgentVersion != "1.0.4" {
		t.Fatalf("AgentVersion = %q, want 1.0.4", got.AgentVersion)
	}
	if len(got.IPs) != 1 || got.IPs[0] != "10.0.0.4" {
		t.Fatalf("IPs = %v, want the json serializer to round-trip [10.0.0.4]", got.IPs)
	}
}
