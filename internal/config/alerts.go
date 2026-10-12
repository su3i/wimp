package config

// AlertConfig holds per-alert-type severity overrides and numeric thresholds. Every
// field is optional (required:"false") - an empty severity override falls back to the
// alert type's registry default; an unset/zero threshold falls back to a hardcoded
// sane default at the call site.
type AlertConfig struct {
	SeverityMachineConnected    string `envconfig:"ALERTSEVERITY_MACHINECONNECTED" required:"false"`
	SeverityMachineDisconnected string `envconfig:"ALERTSEVERITY_MACHINEDISCONNECTED" required:"false"`
	SeverityMachineRestarting   string `envconfig:"ALERTSEVERITY_MACHINERESTARTING" required:"false"`
	SeverityAgentUpdated        string `envconfig:"ALERTSEVERITY_AGENTUPDATED" required:"false"`
	SeverityMachineReassigned   string `envconfig:"ALERTSEVERITY_MACHINEREASSIGNED" required:"false"`

	SeverityAppPoolStopped string `envconfig:"ALERTSEVERITY_APPPOOLSTOPPED" required:"false"`
	SeverityAppPoolStarted string `envconfig:"ALERTSEVERITY_APPPOOLSTARTED" required:"false"`

	SeveritySiteStopped string `envconfig:"ALERTSEVERITY_SITESTOPPED" required:"false"`
	SeveritySiteStarted string `envconfig:"ALERTSEVERITY_SITESTARTED" required:"false"`

	SeverityWindowsExporterDown string `envconfig:"ALERTSEVERITY_WINDOWSEXPORTERDOWN" required:"false"`
	SeverityWindowsExporterUp   string `envconfig:"ALERTSEVERITY_WINDOWSEXPORTERUP" required:"false"`
	SeverityFluentBitDown       string `envconfig:"ALERTSEVERITY_FLUENTBITDOWN" required:"false"`
	SeverityFluentBitUp         string `envconfig:"ALERTSEVERITY_FLUENTBITUP" required:"false"`

	SeverityHealthCheckDown string `envconfig:"ALERTSEVERITY_HEALTHCHECKDOWN" required:"false"`
	SeverityHealthCheckUp   string `envconfig:"ALERTSEVERITY_HEALTHCHECKUP" required:"false"`
	SeverityHealthCheckSlow string `envconfig:"ALERTSEVERITY_HEALTHCHECKSLOW" required:"false"`
	SeverityHealthCheckFast string `envconfig:"ALERTSEVERITY_HEALTHCHECKSLOWRECOVERED" required:"false"`

	SeverityHighCPU             string `envconfig:"ALERTSEVERITY_HIGHCPU" required:"false"`
	SeverityHighCPURecovered    string `envconfig:"ALERTSEVERITY_HIGHCPURECOVERED" required:"false"`
	SeverityHighMemory          string `envconfig:"ALERTSEVERITY_HIGHMEMORY" required:"false"`
	SeverityHighMemoryRecovered string `envconfig:"ALERTSEVERITY_HIGHMEMORYRECOVERED" required:"false"`
	SeverityLowDisk             string `envconfig:"ALERTSEVERITY_LOWDISK" required:"false"`
	SeverityLowDiskRecovered    string `envconfig:"ALERTSEVERITY_LOWDISKRECOVERED" required:"false"`

	SeveritySqlInstanceStopped  string `envconfig:"ALERTSEVERITY_SQLINSTANCESTOPPED" required:"false"`
	SeveritySqlInstanceStarted  string `envconfig:"ALERTSEVERITY_SQLINSTANCESTARTED" required:"false"`
	SeveritySqlAgentStopped     string `envconfig:"ALERTSEVERITY_SQLAGENTSTOPPED" required:"false"`
	SeveritySqlAgentStarted     string `envconfig:"ALERTSEVERITY_SQLAGENTSTARTED" required:"false"`
	SeveritySqlInstanceFailover string `envconfig:"ALERTSEVERITY_SQLINSTANCEFAILOVER" required:"false"`
	SeveritySqlAgFailover       string `envconfig:"ALERTSEVERITY_SQLAGFAILOVER" required:"false"`

	SeveritySqlDatabaseUnhealthy  string `envconfig:"ALERTSEVERITY_SQLDATABASEUNHEALTHY" required:"false"`
	SeveritySqlDatabaseRecovered  string `envconfig:"ALERTSEVERITY_SQLDATABASERECOVERED" required:"false"`
	SeveritySqlBackupOverdue      string `envconfig:"ALERTSEVERITY_SQLBACKUPOVERDUE" required:"false"`
	SeveritySqlBackupRecovered    string `envconfig:"ALERTSEVERITY_SQLBACKUPRECOVERED" required:"false"`
	SeveritySqlLogBackupOverdue   string `envconfig:"ALERTSEVERITY_SQLLOGBACKUPOVERDUE" required:"false"`
	SeveritySqlLogBackupRecovered string `envconfig:"ALERTSEVERITY_SQLLOGBACKUPRECOVERED" required:"false"`
	SeveritySqlJobFailed          string `envconfig:"ALERTSEVERITY_SQLJOBFAILED" required:"false"`
	SeveritySqlJobRecovered       string `envconfig:"ALERTSEVERITY_SQLJOBRECOVERED" required:"false"`
	SeveritySqlAgUnhealthy        string `envconfig:"ALERTSEVERITY_SQLAGUNHEALTHY" required:"false"`
	SeveritySqlAgHealthy          string `envconfig:"ALERTSEVERITY_SQLAGHEALTHY" required:"false"`
	SeveritySqlBlocking           string `envconfig:"ALERTSEVERITY_SQLBLOCKING" required:"false"`
	SeveritySqlBlockingCleared    string `envconfig:"ALERTSEVERITY_SQLBLOCKINGCLEARED" required:"false"`

	SeverityClusterNodeDown string `envconfig:"ALERTSEVERITY_CLUSTERNODEDOWN" required:"false"`
	SeverityClusterNodeUp   string `envconfig:"ALERTSEVERITY_CLUSTERNODEUP" required:"false"`

	ThresholdHighCPUPercent    int `envconfig:"THRESHOLD_HIGHCPU_PERCENT" required:"false"`
	ThresholdHighMemoryPercent int `envconfig:"THRESHOLD_HIGHMEMORY_PERCENT" required:"false"`
	ThresholdLowDiskPercent    int `envconfig:"THRESHOLD_LOWDISK_PERCENT" required:"false"`

	// SQL thresholds. Backup age counts full and differential backups alike, so a weekly
	// full with nightly differentials is healthy under a daily threshold.
	ThresholdSqlBackupHours      int `envconfig:"THRESHOLD_SQL_BACKUP_HOURS" required:"false"`
	ThresholdSqlLogBackupMinutes int `envconfig:"THRESHOLD_SQL_LOG_BACKUP_MINUTES" required:"false"`
	ThresholdSqlBlockingSeconds  int `envconfig:"THRESHOLD_SQL_BLOCKING_SECONDS" required:"false"`

	// ThresholdHealthCheckSlowSeconds is the probe duration above which a responding
	// endpoint counts as slow. Must stay comfortably below the probe timeout (see
	// HealthCheckTimeoutSeconds) or a slow site times out and reports as down before it
	// ever reports as slow.
	ThresholdHealthCheckSlowSeconds int `envconfig:"THRESHOLD_HEALTHCHECK_SLOW_SECONDS" required:"false"`
	// HealthCheckTimeoutSeconds caps how long a single blackbox probe may take before it
	// is recorded as a failure. Prometheus requires a scrape timeout no larger than the
	// scrape interval, so the effective value is also clamped against each application's
	// own health check interval.
	HealthCheckTimeoutSeconds int `envconfig:"HEALTHCHECK_TIMEOUT_SECONDS" required:"false"`

	// ReceiverMinSeverity is the global cutoff: only alerts at or above this severity
	// are forwarded to Alertmanager. Empty resolves to "warning" at the call site.
	ReceiverMinSeverity string `envconfig:"ALERTRECEIVERMINSEVERITY" required:"false"`
}

// SqlThresholdValues are the SQL alert thresholds in effect, with defaults applied. Resolved
// here so the alert checker and the UI, which colours values against them, cannot disagree.
type SqlThresholdValues struct {
	BackupHours      int `json:"backup_hours"`
	LogBackupMinutes int `json:"log_backup_minutes"`
	BlockingSeconds  int `json:"blocking_seconds"`
}

func SqlThresholds() SqlThresholdValues {
	or := func(v, fallback int) int {
		if v > 0 {
			return v
		}
		return fallback
	}
	a := Alerts()
	return SqlThresholdValues{
		BackupHours:      or(a.ThresholdSqlBackupHours, 26),
		LogBackupMinutes: or(a.ThresholdSqlLogBackupMinutes, 120),
		BlockingSeconds:  or(a.ThresholdSqlBlockingSeconds, 60),
	}
}
