package cache

import (
	"sync"
	"time"
)

const poolPendingTTL = 30 * time.Second

type poolEntry struct {
	state     string
	machineID uint
	expiresAt time.Time
}

var (
	poolMu      sync.RWMutex
	poolPending = map[uint]poolEntry{}
)

func SetPoolPending(poolID, machineID uint, state string) {
	poolMu.Lock()
	poolPending[poolID] = poolEntry{state: state, machineID: machineID, expiresAt: time.Now().Add(poolPendingTTL)}
	poolMu.Unlock()
}

func InvalidatePool(poolID uint) {
	poolMu.Lock()
	delete(poolPending, poolID)
	poolMu.Unlock()
}

func InvalidatePoolsByMachine(machineID uint) {
	poolMu.Lock()
	for id, e := range poolPending {
		if e.machineID == machineID {
			delete(poolPending, id)
		}
	}
	poolMu.Unlock()
}

// GetPoolPendingStates returns pending state overrides for the given pool IDs.
// Expired entries are skipped.
func GetPoolPendingStates(poolIDs []uint) map[uint]string {
	poolMu.RLock()
	defer poolMu.RUnlock()
	now := time.Now()
	result := make(map[uint]string)
	for _, id := range poolIDs {
		if e, ok := poolPending[id]; ok && now.Before(e.expiresAt) {
			result[id] = e.state
		}
	}
	return result
}
