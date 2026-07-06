package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
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
			a.logger().Errorf("connection lost: %v - reconnecting in %v", err, backoff)
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
		Type: protocol.TypeRegister,
		Payload: mustMarshal(protocol.RegisterPayload{
			MachineId:      a.cfg.MachineId,
			Hostname:       hostname,
			IPs:            localIPs(),
			Version:        Version,
			WindowsVersion: windowsVersion(),
		}),
	}); err != nil {
		return fmt.Errorf("registration send: %w", err)
	}

	if err := wc.writeJSON(protocol.Message{
		Type:    protocol.TypeHeartbeat,
		Payload: mustMarshal(a.buildHeartbeat()),
	}); err != nil {
		return fmt.Errorf("initial heartbeat send: %w", err)
	}

	hbCtx, cancelHB := context.WithCancel(ctx)
	defer cancelHB()
	go a.heartbeat(hbCtx, wc)

	return a.readLoop(ctx, wc)
}

// buildHeartbeat gathers the current app pool/site/sidecar state. Used both right
// after registering and on every heartbeat tick, so a freshly (re)connected agent
// reports the same snapshot a periodic tick would.
func (a *Agent) buildHeartbeat() protocol.HeartbeatPayload {
	return protocol.HeartbeatPayload{
		MachineId:              a.cfg.MachineId,
		AppPools:               runningAppPools(),
		Sites:                  runningSites(),
		WindowsExporterHealthy: checkWindowsExporterHealthy(),
		FluentBitHealthy:       checkFluentBitHealthy(),
	}
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
				Payload: mustMarshal(a.buildHeartbeat()),
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
			disc := protocol.DiscoveryPayload{
				AppPools: discoverAppPools(),
				Sites:    discoverSites(),
			}
			if err := wc.writeJSON(protocol.Message{
				Type:    protocol.TypeDiscovery,
				Payload: mustMarshal(disc),
			}); err != nil {
				a.logger().Errorf("discovery send: %v", err)
			}

		case protocol.TypeCommand:
			var cmd protocol.CommandPayload
			if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
				continue
			}
			go func(c protocol.CommandPayload) {
				output, err := executeCommand(c.Action, c.TargetType, c.Target)
				result := protocol.CommandResultPayload{
					CommandID: c.CommandID,
					Success:   err == nil,
					Output:    output,
				}
				if err != nil {
					result.Error = err.Error()
				}
				wc.writeJSON(protocol.Message{ //nolint:errcheck
					Type:    protocol.TypeCommandResult,
					Payload: mustMarshal(result),
				})
			}(cmd)

		case protocol.TypeFluentConfig:
			var payload protocol.FluentConfigPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				a.logger().Warningf("bad fluent config payload: %v", err)
				continue
			}
			go func(p protocol.FluentConfigPayload) {
				if err := applyFluentConfig(a.cfg.FluentBitDir, p); err != nil {
					a.logger().Errorf("fluent config apply: %v", err)
				}
			}(payload)

		case protocol.TypeListFiles:
			var req protocol.ListFilesPayload
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				continue
			}
			go func(r protocol.ListFilesPayload) {
				files, err := listFiles(r.Path)
				result := protocol.ListFilesResultPayload{RequestID: r.RequestID, Files: files}
				if err != nil {
					result.Error = err.Error()
				}
				if result.Files == nil {
					result.Files = []string{}
				}
				wc.writeJSON(protocol.Message{ //nolint:errcheck
					Type:    protocol.TypeListFilesResult,
					Payload: mustMarshal(result),
				})
			}(req)

		case protocol.TypeDownloadLogs:
			var req protocol.DownloadLogsPayload
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				continue
			}
			go func(r protocol.DownloadLogsPayload) {
				result := protocol.DownloadLogsResultPayload{RequestID: r.RequestID}
				data, err := zipLogFolder(r.LogPath)
				if err != nil {
					result.Error = err.Error()
				} else {
					result.Data = base64.StdEncoding.EncodeToString(data)
				}
				wc.writeJSON(protocol.Message{ //nolint:errcheck
					Type:    protocol.TypeDownloadLogsResult,
					Payload: mustMarshal(result),
				})
			}(req)

		case protocol.TypeClearLogs:
			var req protocol.ClearLogsPayload
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				continue
			}
			go func(r protocol.ClearLogsPayload) {
				result := protocol.ClearLogsResultPayload{RequestID: r.RequestID}
				deleted, err := clearLogDir(r.LogPath)
				if err != nil {
					result.Error = err.Error()
				}
				result.Deleted = deleted
				wc.writeJSON(protocol.Message{ //nolint:errcheck
					Type:    protocol.TypeClearLogsResult,
					Payload: mustMarshal(result),
				})
			}(req)
		}
	}
}

const (
	maxFolderBytes int64 = 500 * 1024 * 1024 // 500 MB uncompressed
	maxZipBytes    int   = 100 * 1024 * 1024 // 100 MB zipped
)

func zipLogFolder(logFilePath string) ([]byte, error) {
	dir := filepath.Dir(logFilePath)

	var folderSize int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			folderSize += info.Size()
		}
		return nil
	})
	if folderSize > maxFolderBytes {
		return nil, fmt.Errorf("log folder exceeds 500 MB limit (%.0f MB uncompressed)", float64(folderSize)/1024/1024)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		fw, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(fw, f)
		return err
	}); err != nil {
		return nil, fmt.Errorf("zip error: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	if buf.Len() > maxZipBytes {
		return nil, fmt.Errorf("compressed archive exceeds 100 MB limit (%.0f MB)", float64(buf.Len())/1024/1024)
	}

	return buf.Bytes(), nil
}

func clearLogDir(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	var deleted int
	for _, e := range entries {
		if !e.IsDir() {
			if os.Remove(filepath.Join(dirPath, e.Name())) == nil {
				deleted++
			}
		}
	}
	return deleted, nil
}

func listFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, filepath.Join(dirPath, e.Name()))
		}
	}
	return files, nil
}

// safeConn wraps a WebSocket connection with a mutex so the heartbeat goroutine
// and the read loop can both write without racing.
type safeConn struct {
	ws *websocket.Conn
	mu sync.Mutex
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

func localIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
