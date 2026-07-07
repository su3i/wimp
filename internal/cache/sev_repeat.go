package cache

import (
	"sync"
	"time"
)

const sevRepeatInterval = 15 * time.Minute

var (
	sevRepeatMu   sync.Mutex
	sevRepeatLast = map[string]time.Time{}
)

// SevRepeatDue reports whether a sustained Sev-severity breach identified by key should
// re-fire a reminder alert: true at most once per sevRepeatInterval. Records now as the
// new last-fire time whenever it returns true.
func SevRepeatDue(key string) bool {
	sevRepeatMu.Lock()
	defer sevRepeatMu.Unlock()
	now := time.Now()
	if last, ok := sevRepeatLast[key]; ok && now.Sub(last) < sevRepeatInterval {
		return false
	}
	sevRepeatLast[key] = now
	return true
}

// ClearSevRepeat drops a key's repeat timer, called on recovery so a future re-breach
// fires immediately instead of waiting out the old interval.
func ClearSevRepeat(key string) {
	sevRepeatMu.Lock()
	delete(sevRepeatLast, key)
	sevRepeatMu.Unlock()
}
