package protocol

import "encoding/json"

// Message types flowing between agent and control plane.
const (
	TypeRegister      = "register"
	TypeRegisterAck   = "register_ack"
	TypeHeartbeat     = "heartbeat"
	TypeCommand       = "command"
	TypeCommandResult = "command_result"
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

// HeartbeatPayload is sent by the agent every 30 seconds.
type HeartbeatPayload struct {
	MachineId uint `json:"machine_id"`
}
