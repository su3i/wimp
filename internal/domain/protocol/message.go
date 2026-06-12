package protocol

import "encoding/json"

// Message types flowing between agent and control plane.
const (
	TypeRegister      = "register"
	TypeRegisterAck   = "register_ack"
	TypeDiscovery     = "discovery"
	TypeHeartbeat     = "heartbeat"
	TypeCommand       = "command"
	TypeCommandResult = "command_result"
	TypeFluentConfig  = "fluent_config"
)

// Message is the envelope for every WebSocket frame.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RegisterPayload is sent by the agent immediately after connecting.
type RegisterPayload struct {
	MachineId uint     `json:"machine_id"`
	Hostname  string   `json:"hostname"`
	IPs       []string `json:"ips"`
}

// DiscoveryPayload is sent by the agent after RegisterAck with full IIS state.
type DiscoveryPayload struct {
	AppPools []AppPoolInfo `json:"app_pools"`
	Sites    []SiteInfo    `json:"sites"`
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

// HeartbeatPayload contains only the names of currently running app pools and sites.
type HeartbeatPayload struct {
	MachineId uint     `json:"machine_id"`
	AppPools  []string `json:"app_pools"`
	Sites     []string `json:"sites"`
}

// CommandPayload is sent by the control plane to trigger an IIS action.
type CommandPayload struct {
	CommandID  string `json:"command_id"`
	Action     string `json:"action"`      // start, stop, restart, recycle
	TargetType string `json:"target_type"` // app_pool
	Target     string `json:"target"`      // name of the target
}

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
	MachineID      uint              `json:"machine_id"`
	LokiHost       string            `json:"loki_host"`
	LokiPort       string            `json:"loki_port"`
	LokiTlsEnabled bool              `json:"loki_tls_enabled"`
	Configs        []FluentAppConfig `json:"configs"`
}

type FluentAppConfig struct {
	ApplicationID uint   `json:"application_id"`
	PoolID        uint   `json:"pool_id"`
	LogPath       string `json:"log_path"`
}
