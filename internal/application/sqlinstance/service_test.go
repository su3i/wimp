package sqlinstance

import (
	"testing"
	"time"

	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/sqlinstance"
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/protocol"
)

func reset() {
	down = map[string]bool{}
	owners = map[string]string{}
	planned = map[string]time.Time{}
	now = time.Now
}

func fci(state, agent, owner string) []protocol.SqlInstanceInfo {
	return []protocol.SqlInstanceInfo{{Name: "PROD", State: state, AgentState: agent, Clustered: true, VirtualServerName: "SQLVS", OwnerNode: owner}}
}

func types(alerts []Alert) []notification.AlertType {
	out := make([]notification.AlertType, len(alerts))
	for i, a := range alerts {
		out[i] = a.Type
	}
	return out
}

func expect(t *testing.T, step string, got []Alert, want ...notification.AlertType) {
	t.Helper()
	g := types(got)
	if len(g) != len(want) {
		t.Fatalf("%s: want %v, got %v", step, want, g)
	}
	for i := range want {
		if g[i] != want[i] {
			t.Fatalf("%s: want %v, got %v", step, want, g)
		}
	}
}

func TestClusteredInstanceIsReportedOncePerChange(t *testing.T) {
	reset()
	run, ok := protocol.SqlStateRunning, protocol.SqlStateRunning

	// Both nodes report the same FCI. The first sighting is only a baseline.
	expect(t, "baseline A", Evaluate(1, 10, "NODE1", fci(run, ok, "NODE1"), nil))
	expect(t, "baseline B", Evaluate(1, 11, "NODE2", fci(run, ok, "NODE1"), nil))

	// It fails on NODE1. Whichever node reports first raises it, the other stays quiet.
	failed := Evaluate(1, 11, "NODE2", fci(protocol.SqlStateFailed, protocol.SqlStateStopped, "NODE1"), nil)
	expect(t, "fail", failed, notification.AlertSqlInstanceStopped)
	if failed[0].MachineID != 0 || failed[0].Subject != `fci:SQLVS\PROD` {
		t.Fatalf("fci alert must be cluster-scoped, got machine %d subject %q", failed[0].MachineID, failed[0].Subject)
	}
	expect(t, "fail dup", Evaluate(1, 10, "NODE1", fci(protocol.SqlStateFailed, protocol.SqlStateStopped, "NODE1"), nil))

	// The cluster brings it up on NODE2. Recovery and failover are each reported once, and the
	// stopped agent of a failed engine produced no alert of its own.
	back := Evaluate(1, 10, "NODE1", fci(run, ok, "NODE2"), nil)
	expect(t, "recover", back, notification.AlertSqlInstanceFailover, notification.AlertSqlInstanceStarted)
	if back[1].Subject != failed[0].Subject {
		t.Fatalf("recovery must close the same incident subject: %q vs %q", back[1].Subject, failed[0].Subject)
	}
	expect(t, "recover dup", Evaluate(1, 11, "NODE2", fci(run, ok, "NODE2"), nil))

	// Agent stopping while the engine runs is its own alert.
	expect(t, "agent", Evaluate(1, 11, "NODE2", fci(run, protocol.SqlStateStopped, "NODE2"), nil), notification.AlertSqlAgentStopped)
}

func TestStandaloneInstanceIgnoresTransitionalStates(t *testing.T) {
	reset()
	inst := func(state string) []protocol.SqlInstanceInfo {
		return []protocol.SqlInstanceInfo{{Name: "MSSQLSERVER", State: state, AgentState: protocol.SqlStateRunning}}
	}

	expect(t, "baseline", Evaluate(1, 10, "DB01", inst(protocol.SqlStateRunning), nil))
	// A restart seen mid-way is not an outage.
	expect(t, "stopping", Evaluate(1, 10, "DB01", inst(protocol.SqlStateStopping), nil))
	expect(t, "running", Evaluate(1, 10, "DB01", inst(protocol.SqlStateRunning), nil))

	stopped := Evaluate(1, 10, "DB01", inst(protocol.SqlStateStopped), nil)
	expect(t, "stopped", stopped, notification.AlertSqlInstanceStopped)
	if stopped[0].MachineID != 10 || stopped[0].Subject != "MSSQLSERVER" {
		t.Fatalf("standalone alert must be machine-scoped, got machine %d subject %q", stopped[0].MachineID, stopped[0].Subject)
	}
	// Same instance name on another host is a different instance.
	expect(t, "other host", Evaluate(1, 12, "DB02", inst(protocol.SqlStateRunning), nil))
	expect(t, "started", Evaluate(1, 10, "DB01", inst(protocol.SqlStateStarting), nil))
	expect(t, "started", Evaluate(1, 10, "DB01", inst(protocol.SqlStateRunning), nil), notification.AlertSqlInstanceStarted)
}

func TestClusterNodesAndAvailabilityGroups(t *testing.T) {
	reset()
	cl := func(node2, primary string) *protocol.ClusterInfo {
		return &protocol.ClusterInfo{
			Name:               "CLU",
			Nodes:              []protocol.ClusterNodeInfo{{Name: "NODE1", State: "Up"}, {Name: "NODE2", State: node2}},
			AvailabilityGroups: []protocol.AvailabilityGroupInfo{{Name: "AG1", PrimaryNode: primary, State: "Online"}},
		}
	}

	expect(t, "baseline", Evaluate(1, 10, "NODE1", nil, cl("Up", "NODE2")))
	// Paused is deliberate maintenance, not an outage.
	expect(t, "paused", Evaluate(1, 10, "NODE1", nil, cl("Paused", "NODE2")))
	expect(t, "down", Evaluate(1, 10, "NODE1", nil, cl("Down", "NODE1")), notification.AlertClusterNodeDown, notification.AlertSqlAgFailover)
	// The surviving node reports the same change; nothing new.
	expect(t, "down dup", Evaluate(1, 11, "NODE3", nil, cl("Down", "NODE1")))
	expect(t, "up", Evaluate(1, 11, "NODE3", nil, cl("Up", "NODE1")), notification.AlertClusterNodeUp)
}

func TestPlannedRestartIsSilentUnlessItFails(t *testing.T) {
	reset()
	clock := time.Now()
	now = func() time.Time { return clock }
	inst := func(state string) []protocol.SqlInstanceInfo {
		return []protocol.SqlInstanceInfo{{Name: "MSSQLSERVER", State: state, AgentState: protocol.SqlStateRunning}}
	}
	host := machine.Machine{Model: gorm.Model{ID: 10}, Hostname: "DB01"}
	row := sqlinstance.SqlInstance{Name: "MSSQLSERVER"}

	expect(t, "baseline", Evaluate(1, 10, "DB01", inst(protocol.SqlStateRunning), nil))

	// A restart someone asked for: the stop and the start are both expected.
	MarkPlanned(1, host, row)
	expect(t, "planned stop", Evaluate(1, 10, "DB01", inst(protocol.SqlStateStopped), nil))
	expect(t, "planned start", Evaluate(1, 10, "DB01", inst(protocol.SqlStateRunning), nil))
	clock = clock.Add(plannedWindow + time.Second)
	expect(t, "after window", Evaluate(1, 10, "DB01", inst(protocol.SqlStateRunning), nil))

	// A restart that never comes back alerts once the window closes.
	MarkPlanned(1, host, row)
	expect(t, "stuck, in window", Evaluate(1, 10, "DB01", inst(protocol.SqlStateStopped), nil))
	clock = clock.Add(plannedWindow + time.Second)
	expect(t, "stuck, window over", Evaluate(1, 10, "DB01", inst(protocol.SqlStateStopped), nil), notification.AlertSqlInstanceStopped)
}
