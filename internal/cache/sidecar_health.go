package cache

import "sync"

type sidecarHealth struct {
	windowsExporterHealthy bool
	fluentBitHealthy       bool
}

var (
	sidecarMu    sync.Mutex
	sidecarState = map[uint]sidecarHealth{}
)

// known is false on the machine's first-ever report.
func GetSidecarHealth(machineID uint) (windowsExporterHealthy, fluentBitHealthy bool, known bool) {
	sidecarMu.Lock()
	defer sidecarMu.Unlock()
	h, ok := sidecarState[machineID]
	return h.windowsExporterHealthy, h.fluentBitHealthy, ok
}

func SetSidecarHealth(machineID uint, windowsExporterHealthy, fluentBitHealthy bool) {
	sidecarMu.Lock()
	defer sidecarMu.Unlock()
	sidecarState[machineID] = sidecarHealth{
		windowsExporterHealthy: windowsExporterHealthy,
		fluentBitHealthy:       fluentBitHealthy,
	}
}
