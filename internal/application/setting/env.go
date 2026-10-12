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
	case notification.AlertSqlInstanceStopped:
		return a.SeveritySqlInstanceStopped
	case notification.AlertSqlInstanceStarted:
		return a.SeveritySqlInstanceStarted
	case notification.AlertSqlAgentStopped:
		return a.SeveritySqlAgentStopped
	case notification.AlertSqlAgentStarted:
		return a.SeveritySqlAgentStarted
	case notification.AlertSqlInstanceFailover:
		return a.SeveritySqlInstanceFailover
	case notification.AlertSqlAgFailover:
		return a.SeveritySqlAgFailover
	case notification.AlertSqlDatabaseUnhealthy:
		return a.SeveritySqlDatabaseUnhealthy
	case notification.AlertSqlDatabaseRecovered:
		return a.SeveritySqlDatabaseRecovered
	case notification.AlertSqlBackupOverdue:
		return a.SeveritySqlBackupOverdue
	case notification.AlertSqlBackupRecovered:
		return a.SeveritySqlBackupRecovered
	case notification.AlertSqlLogBackupOverdue:
		return a.SeveritySqlLogBackupOverdue
	case notification.AlertSqlLogBackupRecovered:
		return a.SeveritySqlLogBackupRecovered
	case notification.AlertSqlJobFailed:
		return a.SeveritySqlJobFailed
	case notification.AlertSqlJobRecovered:
		return a.SeveritySqlJobRecovered
	case notification.AlertSqlAgUnhealthy:
		return a.SeveritySqlAgUnhealthy
	case notification.AlertSqlAgHealthy:
		return a.SeveritySqlAgHealthy
	case notification.AlertSqlBlocking:
		return a.SeveritySqlBlocking
	case notification.AlertSqlBlockingCleared:
		return a.SeveritySqlBlockingCleared
	case notification.AlertClusterNodeDown:
		return a.SeverityClusterNodeDown
	case notification.AlertClusterNodeUp:
		return a.SeverityClusterNodeUp
	default:
		return ""
	}
}
