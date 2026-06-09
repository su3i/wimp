package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/su3i/wimp/internal/domain/protocol"
)

const (
	heartbeatInterval  = 30 * time.Second
	initialBackoff     = 1 * time.Second
	maxBackoff         = 30 * time.Second
	stableConnDuration = 1 * time.Minute
)

// connect is the outer reconnection loop. It retries forever until ctx is cancelled,
// using exponential backoff that resets after a connection stays up for stableConnDuration.
func (a *Agent) connect(ctx context.Context) {
	backoff := initialBackoff

	for {
		connectedAt := time.Now()

		if err := a.dial(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			if time.Since(connectedAt) >= stableConnDuration {
				backoff = initialBackoff
			}
			a.logger().Errorf("connection lost: %v — reconnecting in %v", err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			backoff = min(backoff*2, maxBackoff)
		}
	}
}

// dial establishes one WebSocket connection, runs the registration + heartbeat + read
// loop, and returns when the connection drops or ctx is cancelled.
func (a *Agent) dial(ctx context.Context) error {
	wsURL, err := buildWSURL(a.cfg.ControlPlaneUrl, a.cfg.RegistrationToken)
	if err != nil {
		return fmt.Errorf("invalid control plane URL: %w", err)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	a.logger().Infof("connected to %s", a.cfg.ControlPlaneUrl)

	wc := &safeConn{ws: conn}

	hostname, _ := os.Hostname()
	if err := wc.writeJSON(protocol.Message{
		Type:    protocol.TypeRegister,
		Payload: mustMarshal(protocol.RegisterPayload{MachineId: a.cfg.MachineId, Hostname: hostname}),
	}); err != nil {
		return fmt.Errorf("registration send: %w", err)
	}

	hbCtx, cancelHB := context.WithCancel(ctx)
	defer cancelHB()
	go a.heartbeat(hbCtx, wc)

	return a.readLoop(ctx, wc)
}

// heartbeat sends a heartbeat message every heartbeatInterval.
func (a *Agent) heartbeat(ctx context.Context, wc *safeConn) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := wc.writeJSON(protocol.Message{
				Type:    protocol.TypeHeartbeat,
				Payload: mustMarshal(protocol.HeartbeatPayload{MachineId: a.cfg.MachineId}),
			})
			if err != nil {
				a.logger().Errorf("heartbeat: %v", err)
				return
			}
		}
	}
}

// readLoop reads inbound control-plane messages until the connection closes.
func (a *Agent) readLoop(ctx context.Context, wc *safeConn) error {
	for {
		_, raw, err := wc.ws.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		var msg protocol.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			a.logger().Warningf("malformed message: %v", err)
			continue
		}

		switch msg.Type {
		case protocol.TypeRegisterAck:
			a.logger().Info("registration acknowledged by control plane")
		case protocol.TypeCommand:
			// TODO: IIS command dispatch
		}
	}
}

// safeConn wraps a WebSocket connection with a mutex so the heartbeat goroutine
// and the read loop can both write without racing.
type safeConn struct {
	ws  *websocket.Conn
	mu  sync.Mutex
}

func (c *safeConn) writeJSON(msg protocol.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteMessage(websocket.TextMessage, data)
}

func buildWSURL(controlPlaneUrl, token string) (string, error) {
	parsed, err := url.Parse(controlPlaneUrl)
	if err != nil {
		return "", err
	}

	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/ws/agent"
	parsed.RawQuery = url.Values{"token": {token}}.Encode()

	return parsed.String(), nil
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
