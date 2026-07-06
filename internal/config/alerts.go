package config

// AlertConfig holds per-alert-type severity overrides and numeric thresholds. Every
// field is optional (required:"false") - an empty severity override falls back to the
// alert type's registry default; an unset/zero threshold falls back to a hardcoded
// sane default at the call site.
type AlertConfig struct {
	SeverityMachineConnected    string `envconfig:"ALERTSEVERITY_MACHINECONNECTED" required:"false"`
	SeverityMachineDisconnected string `envconfig:"ALERTSEVERITY_MACHINEDISCONNECTED" required:"false"`
	SeverityMachineShutdown     string `envconfig:"ALERTSEVERITY_MACHINESHUTDOWN" required:"false"`
	SeverityMachineRestarting   string `envconfig:"ALERTSEVERITY_MACHINERESTARTING" required:"false"`
	SeverityAgentUpdated        string `envconfig:"ALERTSEVERITY_AGENTUPDATED" required:"false"`

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

	SeverityHighCPU             string `envconfig:"ALERTSEVERITY_HIGHCPU" required:"false"`
	SeverityHighCPURecovered    string `envconfig:"ALERTSEVERITY_HIGHCPURECOVERED" required:"false"`
	SeverityHighMemory          string `envconfig:"ALERTSEVERITY_HIGHMEMORY" required:"false"`
	SeverityHighMemoryRecovered string `envconfig:"ALERTSEVERITY_HIGHMEMORYRECOVERED" required:"false"`
	SeverityLowDisk             string `envconfig:"ALERTSEVERITY_LOWDISK" required:"false"`
	SeverityLowDiskRecovered    string `envconfig:"ALERTSEVERITY_LOWDISKRECOVERED" required:"false"`

	ThresholdHighCPUPercent    int `envconfig:"THRESHOLD_HIGHCPU_PERCENT" required:"false"`
	ThresholdHighMemoryPercent int `envconfig:"THRESHOLD_HIGHMEMORY_PERCENT" required:"false"`
	ThresholdLowDiskPercent    int `envconfig:"THRESHOLD_LOWDISK_PERCENT" required:"false"`

	// ReceiverMinSeverity is the global cutoff: only alerts at or above this severity
	// are forwarded to Alertmanager. Empty resolves to "warning" at the call site.
	ReceiverMinSeverity string `envconfig:"ALERTRECEIVERMINSEVERITY" required:"false"`
}
