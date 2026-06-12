package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

// clientHub holds live WebSocket connections from frontend browser clients.
type clientHub struct {
	mu    sync.RWMutex
	conns map[string]*websocket.Conn // keyed by a unique connection ID
}

var (
	clientInstance *clientHub
	clientOnce    sync.Once
)

func Clients() *clientHub {
	clientOnce.Do(func() {
		clientInstance = &clientHub{
			conns: make(map[string]*websocket.Conn),
		}
	})
	return clientInstance
}

func (h *clientHub) Register(id string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[id] = conn
}

func (h *clientHub) Deregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, id)
}

// Broadcast sends a message to all connected frontend clients.
// Stale connections are silently removed.
func (h *clientHub) Broadcast(msg []byte) {
	h.mu.RLock()
	ids := make([]string, 0, len(h.conns))
	for id := range h.conns {
		ids = append(ids, id)
	}
	h.mu.RUnlock()

	for _, id := range ids {
		h.mu.RLock()
		conn, ok := h.conns[id]
		h.mu.RUnlock()
		if !ok {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			h.Deregister(id)
		}
	}
}
