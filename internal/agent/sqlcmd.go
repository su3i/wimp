package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/su3i/wimp/internal/domain/protocol"
)

// executeSqlCommand runs a SQL Server command. A clustered instance is only ever started,
// restarted or moved through the cluster: stopping its service directly would look like a
// failure to the cluster, which would fail it over or restart it out from under the command.
func (a *Agent) executeSqlCommand(ctx context.Context, c protocol.CommandPayload) (string, error) {
	if c.TargetType == protocol.TargetAvailabilityGroup {
		if c.Action != protocol.SqlActionFailover {
			return "", fmt.Errorf("unknown action: %s", c.Action)
		}
		hostname, _ := os.Hostname()
		return a.sqlmon.failoverAvailabilityGroup(ctx, c.Target, hostname)
	}

	inst, ok := a.sqlmon.instance(c.Target)
	if !ok {
		return "", fmt.Errorf("SQL instance %s not found on this host", c.Target)
	}

	if inst.Clustered {
		find := `$r = Get-ClusterResource | Where-Object { $_.ResourceType.Name -eq 'SQL Server' -and ($_ | Get-ClusterParameter -Name InstanceName).Value -eq ` + psQuote(inst.Name) + ` } | Select-Object -First 1
if (-not $r) { throw 'cluster resource for instance not found' }
$g = $r.OwnerGroup.Name
`
		switch c.Action {
		case protocol.SqlActionStart:
			return runPowerShell(find + `Start-ClusterGroup -Name $g | Out-String`)
		case protocol.SqlActionRestart:
			// Taking the engine offline takes its dependent Agent resource with it; starting
			// the group brings both back.
			return runPowerShell(find + `Stop-ClusterResource -Name $r.Name | Out-Null
Start-ClusterGroup -Name $g | Out-String`)
		case protocol.SqlActionMove:
			node := c.Args["node"]
			if node == "" {
				return "", fmt.Errorf("move needs a target node")
			}
			return runPowerShell(find + `Move-ClusterGroup -Name $g -Node ` + psQuote(node) + ` | Out-String`)
		default:
			return "", fmt.Errorf("unknown action: %s", c.Action)
		}
	}

	// SQL Agent is restarted only when it is set to start automatically, so an instance whose
	// Agent is deliberately disabled stays that way.
	startAgent := `$a = Get-Service -Name ` + psQuote(inst.AgentServiceName) + ` -ErrorAction SilentlyContinue
if ($a -and $a.StartType -eq 'Automatic') { Start-Service -InputObject $a }
`
	switch c.Action {
	case protocol.SqlActionStart:
		return runPowerShell(`Start-Service -Name ` + psQuote(inst.ServiceName) + "\n" + startAgent + `'started'`)
	case protocol.SqlActionRestart:
		// -Force stops dependent services (SQL Agent) along with the engine.
		return runPowerShell(`Restart-Service -Name ` + psQuote(inst.ServiceName) + ` -Force
` + startAgent + `'restarted'`)
	case protocol.SqlActionMove:
		return "", fmt.Errorf("instance %s is not clustered", inst.Name)
	default:
		return "", fmt.Errorf("unknown action: %s", c.Action)
	}
}

// failoverAvailabilityGroup performs a planned, no-data-loss failover of ag to the replica on
// this node. It refuses unless this replica is a synchronous-commit secondary whose
// databases are all SYNCHRONIZED: those are exactly the conditions under which SQL Server
// guarantees no data loss. Forced failover is never offered.
func (m *sqlMonitor) failoverAvailabilityGroup(ctx context.Context, ag, hostname string) (string, error) {
	m.mu.Lock()
	cfg, instances := m.cfg, m.instances
	m.mu.Unlock()
	if cfg == nil {
		return "", fmt.Errorf("SQL access is not configured yet")
	}

	for _, inst := range instances {
		if !shouldMonitor(inst, hostname) {
			continue
		}
		if out, held, err := failoverOn(ctx, inst, *cfg, ag); held {
			return out, err
		}
	}
	return "", fmt.Errorf("no running SQL instance on this node holds a replica of %s", ag)
}

// failoverOn attempts the failover on one local instance. held is false when the instance
// holds no replica of ag, so the caller can try the next one.
func failoverOn(ctx context.Context, inst protocol.SqlInstanceInfo, cfg protocol.SqlMonitorConfigPayload, ag string) (string, bool, error) {
	db, err := openInstance(inst, cfg)
	if err != nil {
		return "", false, nil
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, sqlQueryTimeout)
	defer cancel()

	var mode, role, notSynced, dbs int
	err = db.QueryRowContext(ctx, `SELECT ar.availability_mode, rs.role,
				(SELECT COUNT(*) FROM sys.dm_hadr_database_replica_states d WHERE d.replica_id = ar.replica_id AND d.is_local = 1 AND d.synchronization_state <> 2),
				(SELECT COUNT(*) FROM sys.dm_hadr_database_replica_states d WHERE d.replica_id = ar.replica_id AND d.is_local = 1)
			FROM sys.availability_groups ag
			JOIN sys.availability_replicas ar ON ar.group_id = ag.group_id
			JOIN sys.dm_hadr_availability_replica_states rs ON rs.replica_id = ar.replica_id AND rs.is_local = 1
			WHERE ag.name = @p1`, ag).Scan(&mode, &role, &notSynced, &dbs)
	if err != nil {
		return "", false, nil
	}
	switch {
	case role == 1:
		return "", true, fmt.Errorf("this replica is already the primary of %s", ag)
	case mode != 1:
		return "", true, fmt.Errorf("refusing failover: this replica uses asynchronous commit, so failing over to it can lose data")
	case dbs == 0 || notSynced > 0:
		return "", true, fmt.Errorf("refusing failover: %d of %d databases on this replica are not SYNCHRONIZED", notSynced, dbs)
	}
	// The identifier is quoted by SQL Server itself; ag never reaches the batch as text.
	if _, err := db.ExecContext(ctx, `DECLARE @s nvarchar(400) = N'ALTER AVAILABILITY GROUP ' + QUOTENAME(@p1) + N' FAILOVER'; EXEC (@s);`, ag); err != nil {
		return "", true, fmt.Errorf("failover: %w", err)
	}
	return fmt.Sprintf("%s failed over to %s", ag, inst.Name), true, nil
}

func (m *sqlMonitor) instance(name string) (protocol.SqlInstanceInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, i := range m.instances {
		if i.Name == name {
			return i, true
		}
	}
	return protocol.SqlInstanceInfo{}, false
}

func runPowerShell(script string) (string, error) {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"$ErrorActionPreference = 'Stop'\n"+script).CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("%s", output)
	}
	return output, nil
}

// psQuote makes s a single-quoted PowerShell literal, where only ' needs escaping.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
