package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

// clientConn wraps a frontend WebSocket connection with its own write mutex so
// concurrent Broadcast calls cannot race on writes to the same connection.
type clientConn struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (c *clientConn) write(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

// clientHub holds live WebSocket connections from frontend browser clients.
type clientHub struct {
	mu    sync.RWMutex
	conns map[string]*clientConn // keyed by a unique connection ID
}

var (
	clientInstance *clientHub
	clientOnce     sync.Once
)

func Clients() *clientHub {
	clientOnce.Do(func() {
		clientInstance = &clientHub{
			conns: make(map[string]*clientConn),
		}
	})
	return clientInstance
}

func (h *clientHub) Register(id string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[id] = &clientConn{conn: conn}
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
	snapshot := make(map[string]*clientConn, len(h.conns))
	for id, c := range h.conns {
		snapshot[id] = c
	}
	h.mu.RUnlock()

	for id, cc := range snapshot {
		if err := cc.write(msg); err != nil {
			h.Deregister(id)
		}
	}
}
