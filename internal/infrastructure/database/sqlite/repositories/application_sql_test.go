package repositories

import (
	"testing"

	"github.com/su3i/wimp/internal/domain/application"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/sqlinstance"
)

// The health count for SQL members has to follow a clustered instance across nodes, which
// only a real query can show.
func TestSqlMemberHealthFollowsClusteredInstance(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&sqlinstance.SqlInstance{}, &application.ApplicationSqlInstance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, table := range []string{"sql_instances", "application_sql_instances"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("reset %s: %v", table, err)
		}
	}

	node1 := seed(t, db, machine.Machine{Hostname: "NODE1", Status: machine.Online})
	node2 := seed(t, db, machine.Machine{Hostname: "NODE2", Status: machine.Online})
	db1 := seed(t, db, machine.Machine{Hostname: "DB1", Status: machine.Online})

	create := func(i sqlinstance.SqlInstance) sqlinstance.SqlInstance {
		t.Helper()
		if err := db.Create(&i).Error; err != nil {
			t.Fatalf("create instance: %v", err)
		}
		return i
	}
	// The FCI was added from NODE1's row, then failed over to NODE2.
	fciOnNode1 := create(sqlinstance.SqlInstance{MachineID: node1.ID, Name: "PROD", State: "Stopped", Clustered: true, VirtualServerName: "SQLVS"})
	create(sqlinstance.SqlInstance{MachineID: node2.ID, Name: "PROD", State: "Running", Clustered: true, VirtualServerName: "SQLVS"})
	standalone := create(sqlinstance.SqlInstance{MachineID: db1.ID, Name: "MSSQLSERVER", State: "Stopped"})
	// Same instance name as the standalone one, running elsewhere: must not make it healthy.
	create(sqlinstance.SqlInstance{MachineID: node2.ID, Name: "MSSQLSERVER", State: "Running"})

	repo := NewApplicationRepository(db)
	for _, id := range []uint{fciOnNode1.ID, standalone.ID} {
		if err := repo.AddSqlInstance(&application.ApplicationSqlInstance{ApplicationID: 1, SqlInstanceID: id}); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	counts, err := repo.FindSqlCountsByApplicationIDs([]uint{1})
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if got := counts[1]; got.Total != 2 || got.Healthy != 1 {
		t.Fatalf("want 2 members with only the failed-over FCI healthy, got %+v", got)
	}

	// The node it moved to going offline makes it unhealthy, since nothing online runs it.
	if err := db.Model(&machine.Machine{}).Where("id = ?", node2.ID).Update("status", machine.Offline).Error; err != nil {
		t.Fatal(err)
	}
	counts, _ = repo.FindSqlCountsByApplicationIDs([]uint{1})
	if got := counts[1]; got.Healthy != 0 {
		t.Fatalf("want no healthy members once the owning node is offline, got %+v", got)
	}
}
