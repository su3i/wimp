package cache

import "sync"

// sidecarHealth is the last-known health of the per-machine sidecar services.
type sidecarHealth struct {
	windowsExporterHealthy bool
	fluentBitHealthy       bool
}

var (
	sidecarMu    sync.Mutex
	sidecarState = map[uint]sidecarHealth{}
)

// GetSidecarHealth returns the last-known sidecar health for a machine and whether
// one has been recorded yet (false on the machine's first-ever report).
func GetSidecarHealth(machineID uint) (windowsExporterHealthy, fluentBitHealthy bool, known bool) {
	sidecarMu.Lock()
	defer sidecarMu.Unlock()
	h, ok := sidecarState[machineID]
	return h.windowsExporterHealthy, h.fluentBitHealthy, ok
}

// SetSidecarHealth records the latest sidecar health for a machine.
func SetSidecarHealth(machineID uint, windowsExporterHealthy, fluentBitHealthy bool) {
	sidecarMu.Lock()
	defer sidecarMu.Unlock()
	sidecarState[machineID] = sidecarHealth{
		windowsExporterHealthy: windowsExporterHealthy,
		fluentBitHealthy:       fluentBitHealthy,
	}
}
