package agent

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/su3i/wimp/internal/domain/protocol"
)

// localSqlInstance is what the registry says about an installed instance, before any
// runtime state is attached.
type localSqlInstance struct {
	Name              string
	Version           string
	Edition           string
	Port              string
	Clustered         bool
	VirtualServerName string
}

// Cluster resource types the agent reads. Types are fixed English identifiers in the
// cluster API regardless of OS display language.
const (
	resTypeSqlServer = "SQL Server"
	resTypeSqlAgent  = "SQL Server Agent"
	resTypeAG        = "SQL Server Availability Group"
)

type clusterSnapshot struct {
	Name      string
	Nodes     []protocol.ClusterNodeInfo
	Resources []clusterResource
}

type clusterResource struct {
	Name     string
	Type     string
	State    string
	Owner    string
	Instance string // InstanceName parameter, for SQL Server and SQL Server Agent resources
}

// clusterQuery prints the cluster's nodes and its SQL-related resources as one JSON
// object. @() keeps single-element collections serialized as arrays.
const clusterQuery = `$ErrorActionPreference = 'Stop'
Import-Module FailoverClusters
$c = Get-Cluster
$types = @('SQL Server', 'SQL Server Agent', 'SQL Server Availability Group')
[pscustomobject]@{
  Name      = [string]$c.Name
  Nodes     = @(Get-ClusterNode | ForEach-Object { [pscustomobject]@{ name = [string]$_.Name; state = [string]$_.State } })
  Resources = @(Get-ClusterResource | Where-Object { $types -contains $_.ResourceType.Name } | ForEach-Object {
    $inst = ''
    if ($_.ResourceType.Name -ne 'SQL Server Availability Group') {
      $p = $_ | Get-ClusterParameter -Name InstanceName -ErrorAction SilentlyContinue
      if ($p) { $inst = [string]$p.Value }
    }
    [pscustomobject]@{ Name = [string]$_.Name; Type = [string]$_.ResourceType.Name; State = [string]$_.State; Owner = [string]$_.OwnerNode; Instance = $inst }
  })
} | ConvertTo-Json -Depth 4 -Compress`

// queryCluster returns nil on any host that is not a running cluster node.
//
// ponytail: spawns PowerShell on every heartbeat for cluster nodes (~1s CPU per 30s). Move to
// a slower poll, or the MSCluster WMI namespace via a native client, if that shows up on hosts.
func queryCluster() *clusterSnapshot {
	if !serviceRunning("ClusSvc") {
		return nil
	}
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", clusterQuery).Output()
	if err != nil {
		return nil
	}
	var snap clusterSnapshot
	if err := json.Unmarshal(out, &snap); err != nil {
		return nil
	}
	return &snap
}

// discoverSql gathers the current SQL and cluster state. Used for both discovery and every
// heartbeat, so the two always agree.
func discoverSql() ([]protocol.SqlInstanceInfo, *protocol.ClusterInfo) {
	locals := listLocalSqlInstances()
	snap := queryCluster()
	if len(locals) == 0 && snap == nil {
		return nil, nil
	}
	return buildSqlInstances(locals, snap, serviceState), buildClusterInfo(snap)
}

// buildSqlInstances attaches runtime state to each installed instance. A clustered instance
// takes its state from its cluster resources, a standalone one from its Windows services.
func buildSqlInstances(locals []localSqlInstance, snap *clusterSnapshot, svcState func(string) string) []protocol.SqlInstanceInfo {
	result := make([]protocol.SqlInstanceInfo, 0, len(locals))
	for _, l := range locals {
		svc, agentSvc := sqlServiceNames(l.Name)
		info := protocol.SqlInstanceInfo{
			Name:              l.Name,
			ServiceName:       svc,
			AgentServiceName:  agentSvc,
			Version:           l.Version,
			Edition:           l.Edition,
			Port:              l.Port,
			Clustered:         l.Clustered,
			VirtualServerName: l.VirtualServerName,
		}

		engine, agent := findSqlResources(snap, l.Name)
		if l.Clustered && engine != nil {
			info.State = clusterStateToSql(engine.State)
			info.OwnerNode = engine.Owner
			info.AgentState = protocol.SqlStateUnknown
			if agent != nil {
				info.AgentState = clusterStateToSql(agent.State)
			}
		} else {
			// Also the fallback for a clustered instance whose resources could not be read:
			// local service state is wrong on a passive node, but it is the best signal left.
			info.State = serviceStateToSql(svcState(svc))
			info.AgentState = serviceStateToSql(svcState(agentSvc))
		}
		result = append(result, info)
	}
	return result
}

func buildClusterInfo(snap *clusterSnapshot) *protocol.ClusterInfo {
	if snap == nil {
		return nil
	}
	info := &protocol.ClusterInfo{
		Name:               snap.Name,
		Nodes:              snap.Nodes,
		AvailabilityGroups: []protocol.AvailabilityGroupInfo{},
	}
	for _, r := range snap.Resources {
		if r.Type == resTypeAG {
			info.AvailabilityGroups = append(info.AvailabilityGroups, protocol.AvailabilityGroupInfo{
				Name:        r.Name,
				PrimaryNode: r.Owner,
				State:       r.State,
			})
		}
	}
	return info
}

func findSqlResources(snap *clusterSnapshot, instance string) (engine, agent *clusterResource) {
	if snap == nil {
		return nil, nil
	}
	for i := range snap.Resources {
		r := &snap.Resources[i]
		if !strings.EqualFold(r.Instance, instance) {
			continue
		}
		switch r.Type {
		case resTypeSqlServer:
			engine = r
		case resTypeSqlAgent:
			agent = r
		}
	}
	return engine, agent
}

// sqlServiceNames returns the engine and SQL Agent service names for an instance.
func sqlServiceNames(instance string) (string, string) {
	if strings.EqualFold(instance, "MSSQLSERVER") {
		return "MSSQLSERVER", "SQLSERVERAGENT"
	}
	return "MSSQL$" + instance, "SQLAgent$" + instance
}

func serviceStateToSql(s string) string {
	switch s {
	case "RUNNING":
		return protocol.SqlStateRunning
	case "STOPPED":
		return protocol.SqlStateStopped
	case "START_PENDING", "CONTINUE_PENDING":
		return protocol.SqlStateStarting
	case "STOP_PENDING", "PAUSE_PENDING", "PAUSED":
		return protocol.SqlStateStopping
	default:
		return protocol.SqlStateUnknown
	}
}

func clusterStateToSql(s string) string {
	switch s {
	case "Online":
		return protocol.SqlStateRunning
	case "Offline":
		return protocol.SqlStateStopped
	case "OnlinePending", "Pending":
		return protocol.SqlStateStarting
	case "OfflinePending":
		return protocol.SqlStateStopping
	case "Failed":
		return protocol.SqlStateFailed
	default:
		return protocol.SqlStateUnknown
	}
}
