package cache

import (
	"sync"
	"time"
)

const machineActionTTL = 15 * time.Second

type machineActionEntry struct {
	action    string
	expiresAt time.Time
}

var (
	machineActionMu      sync.Mutex
	machineActionPending = map[uint]machineActionEntry{}
)

// SetMachineActionPending marks a machine as having a deliberate shutdown/restart
// in flight, so the WebSocket disconnect handler can tell it apart from a real outage.
func SetMachineActionPending(machineID uint, action string) {
	machineActionMu.Lock()
	machineActionPending[machineID] = machineActionEntry{action: action, expiresAt: time.Now().Add(machineActionTTL)}
	machineActionMu.Unlock()
}

// IsMachineActionPending reports whether the machine has a recent shutdown/restart
// command in flight.
func IsMachineActionPending(machineID uint) bool {
	machineActionMu.Lock()
	defer machineActionMu.Unlock()
	e, ok := machineActionPending[machineID]
	return ok && time.Now().Before(e.expiresAt)
}

func ClearMachineActionPending(machineID uint) {
	machineActionMu.Lock()
	delete(machineActionPending, machineID)
	machineActionMu.Unlock()
}
