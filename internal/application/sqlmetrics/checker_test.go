package sqlmetrics

import (
	"testing"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/config"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	notificationDomain "github.com/su3i/wimp/internal/domain/notification"
)

func setup(t *testing.T) []rule {
	t.Helper()
	for k, v := range map[string]string{
		"APPENV": "test", "APPURL": "http://localhost", "APPPORT": "8080", "LOKIHOST": "loki",
		"LOKIPORT": "3100", "BOOTSTRAPTOKEN": "token", "AGENTVERSION": "1.0.0", "JWTSECRET": "secret",
		"DEFAULTADMINUSERNAME": "admin", "DEFAULTADMINPASSWORD": "password",
	} {
		t.Setenv(k, v)
	}
	config.Initialize()
	mu.Lock()
	breaches = map[string]bool{}
	mu.Unlock()
	return rules()
}

func ruleFor(t *testing.T, rs []rule, a notificationDomain.AlertType) rule {
	t.Helper()
	for _, r := range rs {
		if r.alert == a {
			return r
		}
	}
	t.Fatalf("no rule for %s", a)
	return rule{}
}

var machines = map[string]machineDomain.Machine{
	"10": {Model: gorm.Model{ID: 10}, ProjectID: 1, Hostname: "NODE1"},
	"11": {Model: gorm.Model{ID: 11}, ProjectID: 1, Hostname: "NODE2"},
}

const day = 24 * 3600.0

func dbSample(machineID, server, db, ag, clustered string, age float64) sample {
	return sample{labels: map[string]string{"machine_id": machineID, "sql_server": server, "database": db,
		"availability_group": ag, "clustered": clustered}, value: age}
}

func TestAvailabilityGroupBackupCountsAcrossReplicas(t *testing.T) {
	r := ruleFor(t, setup(t), notificationDomain.AlertSqlBackupOverdue)

	// Backups run on the secondary only. The primary's msdb has none, which must not alert.
	got := evaluate(r, []sample{
		dbSample("10", "NODE1", "Sales", "AG1", "false", 30*day),
		dbSample("11", "NODE2", "Sales", "AG1", "false", 2*3600),
	}, machines)
	if len(got) != 0 {
		t.Fatalf("a fresh backup on any replica covers the AG database, got %+v", got)
	}

	// Now no replica has one.
	got = evaluate(r, []sample{
		dbSample("10", "NODE1", "Sales", "AG1", "false", 30*day),
		dbSample("11", "NODE2", "Sales", "AG1", "false", 2*day),
	}, machines)
	if len(got) != 1 || got[0].machineID != 0 || got[0].subject != "ag:AG1/Sales" || got[0].instance != "AG1" {
		t.Fatalf("want one AG-scoped alert, got %+v", got)
	}
}

func TestStandaloneBackupEdgeTriggers(t *testing.T) {
	r := ruleFor(t, setup(t), notificationDomain.AlertSqlBackupOverdue)
	overdue := []sample{dbSample("10", `NODE1\PROD`, "Orders", "", "false", 3*day)}

	got := evaluate(r, overdue, machines)
	if len(got) != 1 || got[0].alertType != notificationDomain.AlertSqlBackupOverdue || got[0].machineID != 10 {
		t.Fatalf("want one machine-scoped overdue alert, got %+v", got)
	}
	if again := evaluate(r, overdue, machines); len(again) != 0 {
		t.Fatalf("a sustained breach below Sev must not re-alert, got %+v", again)
	}
	got = evaluate(r, []sample{dbSample("10", `NODE1\PROD`, "Orders", "", "false", 600)}, machines)
	if len(got) != 1 || got[0].alertType != notificationDomain.AlertSqlBackupRecovered || got[0].subject != `NODE1\PROD/Orders` {
		t.Fatalf("want recovery on the same subject, got %+v", got)
	}
}

func TestClusteredInstanceAlertsAreNotMachineScoped(t *testing.T) {
	r := ruleFor(t, setup(t), notificationDomain.AlertSqlJobFailed)
	l := map[string]string{"machine_id": "10", "sql_server": `SQLVS\PROD`, "clustered": "true", "job": "Nightly"}

	got := evaluate(r, []sample{{labels: l, value: 1}}, machines)
	if len(got) != 1 || got[0].machineID != 0 {
		t.Fatalf("an FCI job must alert cluster-scoped, got %+v", got)
	}
	// After failover the same job is reported from the other node: same incident, no new alert.
	l2 := map[string]string{"machine_id": "11", "sql_server": `SQLVS\PROD`, "clustered": "true", "job": "Nightly"}
	if again := evaluate(r, []sample{{labels: l2, value: 1}}, machines); len(again) != 0 {
		t.Fatalf("failover must not re-alert an ongoing breach, got %+v", again)
	}
}
