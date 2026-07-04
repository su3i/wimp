package cache

import (
	"sync"
	"time"
)

const sitePendingTTL = 30 * time.Second

type siteEntry struct {
	state     string
	machineID uint
	expiresAt time.Time
}

var (
	siteMu      sync.RWMutex
	sitePending = map[uint]siteEntry{}
)

func SetSitePending(siteID, machineID uint, state string) {
	siteMu.Lock()
	sitePending[siteID] = siteEntry{state: state, machineID: machineID, expiresAt: time.Now().Add(sitePendingTTL)}
	siteMu.Unlock()
}

func InvalidateSite(siteID uint) {
	siteMu.Lock()
	delete(sitePending, siteID)
	siteMu.Unlock()
}

func InvalidateSitesByMachine(machineID uint) {
	siteMu.Lock()
	for id, e := range sitePending {
		if e.machineID == machineID {
			delete(sitePending, id)
		}
	}
	siteMu.Unlock()
}

// GetSitePendingStates returns pending state overrides for the given site IDs.
// Expired entries are skipped.
func GetSitePendingStates(siteIDs []uint) map[uint]string {
	siteMu.RLock()
	defer siteMu.RUnlock()
	now := time.Now()
	result := make(map[uint]string)
	for _, id := range siteIDs {
		if e, ok := sitePending[id]; ok && now.Before(e.expiresAt) {
			result[id] = e.state
		}
	}
	return result
}
