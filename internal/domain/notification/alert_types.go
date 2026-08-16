package notification

// AlertType is a stable identifier for every alert-worthy event in the system,
// independent of the free-text Title/Detail shown to users. This registry is the
// single source of truth for each type's category and default severity - the
// exhaustive list of everything that can alert.
type AlertType string

const (
	AlertMachineConnected    AlertType = "machine_connected"
	AlertMachineDisconnected AlertType = "machine_disconnected" // unplanned
	AlertMachineShutdown     AlertType = "machine_shutdown"     // planned
	AlertMachineRestarting   AlertType = "machine_restarting"   // planned
	AlertAgentUpdated        AlertType = "agent_updated"
	// AlertMachineReassigned fires in the project a host just left, when that same
	// physical box re-registers against a different machine row.
	AlertMachineReassigned AlertType = "machine_reassigned"

	AlertAppPoolStopped AlertType = "app_pool_stopped"
	AlertAppPoolStarted AlertType = "app_pool_started"

	AlertSiteStopped AlertType = "site_stopped"
	AlertSiteStarted AlertType = "site_started"

	AlertWindowsExporterDown AlertType = "windows_exporter_down"
	AlertWindowsExporterUp   AlertType = "windows_exporter_recovered"
	AlertFluentBitDown       AlertType = "fluent_bit_down"
	AlertFluentBitUp         AlertType = "fluent_bit_recovered"

	AlertHealthCheckDown AlertType = "health_check_down"
	AlertHealthCheckUp   AlertType = "health_check_recovered"
	// AlertHealthCheckSlow separates "responding, but too slowly to be healthy" from
	// AlertHealthCheckDown ("not responding at all"). Without it a slow site trips the
	// down alert, because a probe that exceeds its timeout is indistinguishable from a
	// probe that got no answer.
	AlertHealthCheckSlow AlertType = "health_check_slow"
	AlertHealthCheckFast AlertType = "health_check_slow_recovered"

	AlertHighCPU             AlertType = "high_cpu"
	AlertHighCPURecovered    AlertType = "high_cpu_recovered"
	AlertHighMemory          AlertType = "high_memory"
	AlertHighMemoryRecovered AlertType = "high_memory_recovered"
	AlertLowDisk             AlertType = "low_disk"
	AlertLowDiskRecovered    AlertType = "low_disk_recovered"
)

type AlertTypeMeta struct {
	Category        Category
	DefaultSeverity Level
}

// AlertTypeRegistry maps every AlertType to its category and default severity. A
// severity configured via env (see internal/config/alerts.go) overrides the default
// here; if a type is set to Disabled, EmitAlert skips it entirely.
var AlertTypeRegistry = map[AlertType]AlertTypeMeta{
	AlertMachineConnected:    {CategoryMachine, LevelInfo},
	AlertMachineDisconnected: {CategoryMachine, LevelSev},
	AlertMachineShutdown:     {CategoryMachine, LevelInfo},
	AlertMachineRestarting:   {CategoryMachine, LevelInfo},
	AlertAgentUpdated:        {CategoryMachine, LevelInfo},
	AlertMachineReassigned:   {CategoryMachine, LevelWarning},

	AlertAppPoolStopped: {CategoryAppPool, LevelCritical},
	AlertAppPoolStarted: {CategoryAppPool, LevelInfo},

	AlertSiteStopped: {CategoryIIS, LevelCritical},
	AlertSiteStarted: {CategoryIIS, LevelInfo},

	AlertWindowsExporterDown: {CategorySidecar, LevelCritical},
	AlertWindowsExporterUp:   {CategorySidecar, LevelInfo},
	AlertFluentBitDown:       {CategorySidecar, LevelCritical},
	AlertFluentBitUp:         {CategorySidecar, LevelInfo},

	AlertHealthCheckDown: {CategoryService, LevelCritical},
	AlertHealthCheckUp:   {CategoryService, LevelInfo},
	AlertHealthCheckSlow: {CategoryService, LevelCritical},
	AlertHealthCheckFast: {CategoryService, LevelInfo},

	AlertHighCPU:             {CategoryMetrics, LevelWarning},
	AlertHighCPURecovered:    {CategoryMetrics, LevelInfo},
	AlertHighMemory:          {CategoryMetrics, LevelWarning},
	AlertHighMemoryRecovered: {CategoryMetrics, LevelInfo},
	AlertLowDisk:             {CategoryMetrics, LevelCritical},
	AlertLowDiskRecovered:    {CategoryMetrics, LevelInfo},
}
