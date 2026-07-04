package hub

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

// agentConn wraps a WebSocket connection with its own write mutex so concurrent
// HTTP handlers (e.g. two simultaneous app-pool commands) cannot race on writes.
// gorilla/websocket allows one concurrent reader and one concurrent writer; the
// read loop in ws.go is the sole reader, so only writes need the extra mutex.
type agentConn struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (a *agentConn) write(msg []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn.WriteMessage(websocket.TextMessage, msg)
}

// Hub holds the live WebSocket connections from all agents, keyed by machine ID.
type Hub struct {
	mu    sync.RWMutex
	conns map[uint]*agentConn
}

var (
	instance *Hub
	once     sync.Once
)

func Get() *Hub {
	once.Do(func() {
		instance = &Hub{
			conns: make(map[uint]*agentConn),
		}
	})
	return instance
}

func (h *Hub) Register(machineID uint, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[machineID] = &agentConn{conn: conn}
}

func (h *Hub) Deregister(machineID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, machineID)
}

func (h *Hub) IsOnline(machineID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[machineID]
	return ok
}

// Send writes a raw message to a connected agent. Returns an error if the
// machine is not currently connected.
func (h *Hub) Send(machineID uint, msg []byte) error {
	h.mu.RLock()
	ac, ok := h.conns[machineID]
	h.mu.RUnlock()

	if !ok {
		return errors.New("machine not connected")
	}

	return ac.write(msg)
}
