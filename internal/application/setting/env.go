package setting

import (
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
)

// envSeverity returns the severity the deployment's environment sets for an alert type,
// or "" if it sets none. One explicit case per alert type - deliberately not
// reflection-based, so every mapping is greppable and traceable.
func envSeverity(alertType notification.AlertType) string {
	a := config.Alerts()
	switch alertType {
	case notification.AlertMachineConnected:
		return a.SeverityMachineConnected
	case notification.AlertMachineDisconnected:
		return a.SeverityMachineDisconnected
	case notification.AlertMachineShutdown:
		return a.SeverityMachineShutdown
	case notification.AlertMachineRestarting:
		return a.SeverityMachineRestarting
	case notification.AlertAgentUpdated:
		return a.SeverityAgentUpdated
	case notification.AlertMachineReassigned:
		return a.SeverityMachineReassigned
	case notification.AlertAppPoolStopped:
		return a.SeverityAppPoolStopped
	case notification.AlertAppPoolStarted:
		return a.SeverityAppPoolStarted
	case notification.AlertSiteStopped:
		return a.SeveritySiteStopped
	case notification.AlertSiteStarted:
		return a.SeveritySiteStarted
	case notification.AlertWindowsExporterDown:
		return a.SeverityWindowsExporterDown
	case notification.AlertWindowsExporterUp:
		return a.SeverityWindowsExporterUp
	case notification.AlertFluentBitDown:
		return a.SeverityFluentBitDown
	case notification.AlertFluentBitUp:
		return a.SeverityFluentBitUp
	case notification.AlertHealthCheckDown:
		return a.SeverityHealthCheckDown
	case notification.AlertHealthCheckUp:
		return a.SeverityHealthCheckUp
	case notification.AlertHealthCheckSlow:
		return a.SeverityHealthCheckSlow
	case notification.AlertHealthCheckFast:
		return a.SeverityHealthCheckFast
	case notification.AlertHighCPU:
		return a.SeverityHighCPU
	case notification.AlertHighCPURecovered:
		return a.SeverityHighCPURecovered
	case notification.AlertHighMemory:
		return a.SeverityHighMemory
	case notification.AlertHighMemoryRecovered:
		return a.SeverityHighMemoryRecovered
	case notification.AlertLowDisk:
		return a.SeverityLowDisk
	case notification.AlertLowDiskRecovered:
		return a.SeverityLowDiskRecovered
	default:
		return ""
	}
}
