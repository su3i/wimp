package hub

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub holds the live WebSocket connections from all agents, keyed by machine ID.
type Hub struct {
	mu    sync.RWMutex
	conns map[uint]*websocket.Conn
}

var (
	instance *Hub
	once     sync.Once
)

func Get() *Hub {
	once.Do(func() {
		instance = &Hub{
			conns: make(map[uint]*websocket.Conn),
		}
	})
	return instance
}

func (h *Hub) Register(machineID uint, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[machineID] = conn
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
	conn, ok := h.conns[machineID]
	h.mu.RUnlock()

	if !ok {
		return errors.New("machine not connected")
	}

	return conn.WriteMessage(websocket.TextMessage, msg)
}
