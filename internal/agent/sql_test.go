package agent

import (
	"encoding/json"
	"testing"

	"github.com/su3i/wimp/internal/domain/protocol"
)

// Shape of clusterQuery's output, as ConvertTo-Json prints it.
const sampleClusterJSON = `{"Name":"SQLCLU01","Nodes":[{"name":"NODE1","state":"Up"},{"name":"NODE2","state":"Down"}],"Resources":[{"Name":"SQL Server (PROD)","Type":"SQL Server","State":"Online","Owner":"NODE1","Instance":"PROD"},{"Name":"SQL Server Agent (PROD)","Type":"SQL Server Agent","State":"Failed","Owner":"NODE1","Instance":"PROD"},{"Name":"AG_SALES","Type":"SQL Server Availability Group","State":"Online","Owner":"NODE2","Instance":""}]}`

func TestBuildSqlInstances(t *testing.T) {
	var snap clusterSnapshot
	if err := json.Unmarshal([]byte(sampleClusterJSON), &snap); err != nil {
		t.Fatalf("cluster json: %v", err)
	}

	locals := []localSqlInstance{
		{Name: "PROD", Clustered: true, VirtualServerName: "SQLPRODVS"},
		{Name: "MSSQLSERVER"},
	}
	// This is a passive FCI node: every local SQL service is stopped. The clustered instance
	// must still read as running, from its cluster resource.
	svc := func(name string) string {
		if name == "MSSQLSERVER" {
			return "RUNNING"
		}
		return "STOPPED"
	}

	got := buildSqlInstances(locals, &snap, svc)
	if len(got) != 2 {
		t.Fatalf("want 2 instances, got %d", len(got))
	}

	fci := got[0]
	if fci.ServiceName != "MSSQL$PROD" || fci.AgentServiceName != "SQLAgent$PROD" {
		t.Errorf("named instance services: %q %q", fci.ServiceName, fci.AgentServiceName)
	}
	if fci.State != protocol.SqlStateRunning || fci.AgentState != protocol.SqlStateFailed || fci.OwnerNode != "NODE1" {
		t.Errorf("fci state from cluster: state=%q agent=%q owner=%q", fci.State, fci.AgentState, fci.OwnerNode)
	}

	std := got[1]
	if std.ServiceName != "MSSQLSERVER" || std.AgentServiceName != "SQLSERVERAGENT" {
		t.Errorf("default instance services: %q %q", std.ServiceName, std.AgentServiceName)
	}
	if std.State != protocol.SqlStateRunning || std.AgentState != protocol.SqlStateStopped || std.OwnerNode != "" {
		t.Errorf("standalone state from services: state=%q agent=%q owner=%q", std.State, std.AgentState, std.OwnerNode)
	}

	cl := buildClusterInfo(&snap)
	if cl.Name != "SQLCLU01" || len(cl.Nodes) != 2 || cl.Nodes[1].State != "Down" {
		t.Errorf("cluster info: %+v", cl)
	}
	if len(cl.AvailabilityGroups) != 1 || cl.AvailabilityGroups[0].Name != "AG_SALES" || cl.AvailabilityGroups[0].PrimaryNode != "NODE2" {
		t.Errorf("availability groups: %+v", cl.AvailabilityGroups)
	}
}

func TestConnStringTargetsTheRightEndpoint(t *testing.T) {
	cases := []struct {
		inst protocol.SqlInstanceInfo
		cfg  protocol.SqlMonitorConfigPayload
		want string
	}{
		// Default instance, Windows auth: no user in the URL.
		{protocol.SqlInstanceInfo{Name: "MSSQLSERVER", Port: "1433"}, protocol.SqlMonitorConfigPayload{},
			"sqlserver://localhost:1433?TrustServerCertificate=true&app+name=wimp-agent&database=master&dial+timeout=10"},
		// Named instance without a fixed port goes through SQL Browser.
		{protocol.SqlInstanceInfo{Name: "REPORTS", Port: "0"}, protocol.SqlMonitorConfigPayload{Username: "wimp", Password: "p@ss"},
			"sqlserver://wimp:p%40ss@localhost/REPORTS?TrustServerCertificate=true&app+name=wimp-agent&database=master&dial+timeout=10"},
		// An FCI is reached on its virtual name, never localhost.
		{protocol.SqlInstanceInfo{Name: "PROD", Port: "50001", Clustered: true, VirtualServerName: "SQLVS"}, protocol.SqlMonitorConfigPayload{},
			"sqlserver://SQLVS:50001?TrustServerCertificate=true&app+name=wimp-agent&database=master&dial+timeout=10"},
	}
	for _, c := range cases {
		if got := connString(c.inst, c.cfg); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.inst.Name, got, c.want)
		}
	}
}

func TestPromWriterEscapesLabels(t *testing.T) {
	w := newPromWriter()
	w.add("wimp_mssql_job_last_run_failed", "gauge", "h", []label{{"job", `Nightly "full" \ backup`}}, 1)
	want := "# HELP wimp_mssql_job_last_run_failed h\n# TYPE wimp_mssql_job_last_run_failed gauge\n" +
		`wimp_mssql_job_last_run_failed{job="Nightly \"full\" \\ backup"} 1` + "\n"
	if got := w.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestOnlyOneNodeMonitorsAnInstance(t *testing.T) {
	running := protocol.SqlInstanceInfo{Name: "PROD", State: protocol.SqlStateRunning, Clustered: true, OwnerNode: "NODE1"}
	if !shouldMonitor(running, "node1") || shouldMonitor(running, "NODE2") {
		t.Error("an FCI must be monitored by its owner node only")
	}
	if shouldMonitor(protocol.SqlInstanceInfo{Name: "X", State: protocol.SqlStateStopped}, "H") {
		t.Error("a stopped instance cannot be queried")
	}
}
