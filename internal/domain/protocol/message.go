package protocol

import "encoding/json"

// Message types flowing between agent and control plane.
const (
	TypeRegister       = "register"
	TypeRegisterAck    = "register_ack"
	TypeDiscovery      = "discovery"
	TypeHeartbeat      = "heartbeat"
	TypeCommand        = "command"
	TypeCommandResult  = "command_result"
	TypeFluentConfig   = "fluent_config"
	TypeListFiles          = "list_files"
	TypeListFilesResult    = "list_files_result"
	TypeDownloadLogs       = "download_logs"
	TypeDownloadLogsResult = "download_logs_result"
	TypeClearLogs          = "clear_logs"
	TypeClearLogsResult    = "clear_logs_result"
	TypeSqlMonitorConfig   = "sql_monitor_config"
	TypeSqlQuery           = "sql_query"
	TypeSqlQueryResult     = "sql_query_result"
)

// Message is the envelope for every WebSocket frame.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RegisterPayload is sent by the agent immediately after connecting.
type RegisterPayload struct {
	MachineId      uint     `json:"machine_id"`
	Hostname       string   `json:"hostname"`
	IPs            []string `json:"ips"`
	Version        string   `json:"version"`
	WindowsVersion string   `json:"windows_version"`
	// MachineUID is a stable identifier for the physical box (Windows MachineGuid). It
	// survives re-bootstrapping into a different project, which the machine id and token
	// do not, and is what lets the control plane recognize a host it already knows under
	// a different row. Empty when it can't be read, and empty from agents older than the
	// build that started sending it.
	MachineUID string `json:"machine_uid,omitempty"`
}

// DiscoveryPayload is sent by the agent after RegisterAck with everything it found running.
// Every section is empty on a host without that workload, which is how the control plane
// learns what a host is for without the operator having to say.
type DiscoveryPayload struct {
	AppPools     []AppPoolInfo     `json:"app_pools"`
	Sites        []SiteInfo        `json:"sites"`
	SqlInstances []SqlInstanceInfo `json:"sql_instances"`
	// Cluster is nil unless the host is a Windows Server Failover Cluster node.
	Cluster *ClusterInfo `json:"cluster,omitempty"`
}

// SQL instance states, normalized across standalone services and clustered resources so the
// control plane compares one vocabulary regardless of topology.
const (
	SqlStateRunning  = "Running"
	SqlStateStopped  = "Stopped"
	SqlStateStarting = "Starting"
	SqlStateStopping = "Stopping"
	SqlStateFailed   = "Failed"
	SqlStateUnknown  = "Unknown"
)

type SqlInstanceInfo struct {
	// Name is the instance name, MSSQLSERVER for the default instance.
	Name             string `json:"name"`
	ServiceName      string `json:"service_name"`
	AgentServiceName string `json:"agent_service_name"`
	Version          string `json:"version"`
	Edition          string `json:"edition"`
	Port             string `json:"port"`
	// State and AgentState come from the cluster resource for a clustered instance (FCI) and
	// from the local Windows service otherwise. On a passive FCI node the local service is
	// always stopped, so reading it there would report a healthy instance as down.
	State      string `json:"state"`
	AgentState string `json:"agent_state"`
	// Clustered marks a failover cluster instance. VirtualServerName is its network name and
	// OwnerNode the node currently running it.
	Clustered         bool   `json:"clustered"`
	VirtualServerName string `json:"virtual_server_name,omitempty"`
	OwnerNode         string `json:"owner_node,omitempty"`

	// Monitored is true when this node is the one querying the instance's DMVs: it is
	// running here (or this node owns the FCI). MonitorError is the last query failure, ""
	// when the last poll succeeded. A login or permission failure here is what the UI
	// turns into "configure SQL credentials".
	Monitored    bool   `json:"monitored"`
	MonitorError string `json:"monitor_error,omitempty"`
}

// SqlMonitorConfigPayload is pushed to the agent on every register. An empty Username means
// Windows authentication as the agent's own service identity.
type SqlMonitorConfigPayload struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// SQL live-query kinds: point-in-time lists that are not time series and so do not belong in
// Prometheus. Fetched on demand when someone is looking.
const (
	SqlQueryBlocking = "blocking"
	SqlQueryRequests = "requests"
	SqlQueryJobs     = "jobs"
)

type SqlQueryPayload struct {
	RequestID string `json:"request_id"`
	Instance  string `json:"instance"`
	Kind      string `json:"kind"`
}

type SqlQueryResultPayload struct {
	RequestID string           `json:"request_id"`
	Rows      []map[string]any `json:"rows,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type ClusterInfo struct {
	// Every node reports the same cluster-wide view; the control plane deduplicates.
	Name               string                  `json:"name"`
	Nodes              []ClusterNodeInfo       `json:"nodes"`
	AvailabilityGroups []AvailabilityGroupInfo `json:"availability_groups"`
}

type ClusterNodeInfo struct {
	Name  string `json:"name"`
	State string `json:"state"` // Up, Down, Paused, Joining
}

// AvailabilityGroupInfo is read from the AG's cluster resource, whose owner node is the
// primary replica. That makes AGs discoverable without SQL credentials.
type AvailabilityGroupInfo struct {
	Name        string `json:"name"`
	PrimaryNode string `json:"primary_node"`
	State       string `json:"state"`
}

type AppPoolInfo struct {
	Name           string `json:"name"`
	State          string `json:"state"`
	RuntimeVersion string `json:"runtime_version"`
	PipelineMode   string `json:"pipeline_mode"`
	StartMode      string `json:"start_mode"`
	IdentityType   string `json:"identity_type"`
}

type SiteInfo struct {
	Name         string        `json:"name"`
	State        string        `json:"state"`
	PhysicalPath string        `json:"physical_path"`
	AppPoolName  string        `json:"app_pool_name"`
	Bindings     []BindingInfo `json:"bindings"`
}

type BindingInfo struct {
	Protocol string `json:"protocol"`
	IP       string `json:"ip"`
	Port     string `json:"port"`
	Hostname string `json:"hostname"`
}

// HeartbeatPayload contains the names of currently running app pools and sites, and the
// full SQL and cluster snapshot: SQL state is more than running/not running (agent state,
// owner node, AG primary), so names alone cannot carry it.
type HeartbeatPayload struct {
	MachineId              uint              `json:"machine_id"`
	AppPools               []string          `json:"app_pools"`
	Sites                  []string          `json:"sites"`
	SqlInstances           []SqlInstanceInfo `json:"sql_instances"`
	Cluster                *ClusterInfo      `json:"cluster,omitempty"`
	WindowsExporterHealthy bool              `json:"windows_exporter_healthy"`
	FluentBitHealthy       bool              `json:"fluent_bit_healthy"`
}

// CommandPayload is sent by the control plane to trigger an IIS action.
type CommandPayload struct {
	CommandID  string `json:"command_id"`
	Action     string `json:"action"`      // start, stop, restart, recycle, update, move, failover
	TargetType string `json:"target_type"` // app_pool, site, machine, agent
	Target     string `json:"target"`      // name of the target (unused for machine); download URL for agent update
	// Args carries what an action needs beyond its target, such as the node to move a
	// clustered instance to.
	Args map[string]string `json:"args,omitempty"`
}

// SQL command target types and actions. Stopping an instance is deliberately not offered.
const (
	TargetSqlInstance       = "sql_instance"
	TargetAvailabilityGroup = "availability_group"

	SqlActionStart    = "start"
	SqlActionRestart  = "restart"
	SqlActionMove     = "move"     // FCI to another node; Args["node"]
	SqlActionFailover = "failover" // planned AG failover, run on the target replica
)

// CommandResultPayload is sent by the agent after executing a command.
type CommandResultPayload struct {
	CommandID string `json:"command_id"`
	Success   bool   `json:"success"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
}

// FluentConfigPayload is sent by the control plane to configure fluent-bit log tailing.
// It carries the full set of active pool log configs for a machine; the agent reconciles
// its conf.d directory against this list on every receive.
type FluentConfigPayload struct {
	MachineID         uint              `json:"machine_id"`
	LokiHost          string            `json:"loki_host"`
	LokiPort          string            `json:"loki_port"`
	LokiTlsEnabled    bool              `json:"loki_tls_enabled"`
	LokiTlsSkipVerify bool              `json:"loki_tls_skip_verify"`
	Configs           []FluentAppConfig `json:"configs"`
}

type FluentAppConfig struct {
	ApplicationID uint   `json:"application_id"`
	PoolID        uint   `json:"pool_id"`
	LogPath       string `json:"log_path"`
}

type ListFilesPayload struct {
	RequestID string `json:"request_id"`
	Path      string `json:"path"`
}

type ListFilesResultPayload struct {
	RequestID string   `json:"request_id"`
	Files     []string `json:"files"`
	Error     string   `json:"error,omitempty"`
}

// DownloadLogsPayload is sent by the control plane to request a zipped log folder.
// LogPath is the full path to any file in the log directory; the agent zips the parent directory.
type DownloadLogsPayload struct {
	RequestID string `json:"request_id"`
	LogPath   string `json:"log_path"`
}

// DownloadLogsResultPayload is sent by the agent with the base64-encoded zip archive.
type DownloadLogsResultPayload struct {
	RequestID string `json:"request_id"`
	Data      string `json:"data,omitempty"` // base64-encoded zip bytes
	Error     string `json:"error,omitempty"`
}

// ClearLogsPayload is sent by the control plane to delete all files in a log directory.
type ClearLogsPayload struct {
	RequestID string `json:"request_id"`
	LogPath   string `json:"log_path"` // directory path
}

// ClearLogsResultPayload is sent by the agent after deleting log files.
type ClearLogsResultPayload struct {
	RequestID string `json:"request_id"`
	Deleted   int    `json:"deleted"`
	Error     string `json:"error,omitempty"`
}
