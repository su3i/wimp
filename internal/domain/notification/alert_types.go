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

// ── Incident bindings ──────────────────────────────────────────────────────────
//
// Which alerts begin and end a fault condition. This lives beside AlertTypeRegistry
// deliberately: adding an alert type without deciding whether it opens, closes, or does
// neither is exactly the omission that leaves incidents dangling, and keeping both tables
// in one file makes that omission visible in review.

// IncidentKind names a condition. The alert that opens an incident and the alert that
// closes it share one, and that shared value is how a recovery finds its failure.
type IncidentKind string

const (
	IncidentMachineDown     IncidentKind = "machine_down"
	IncidentAppPoolDown     IncidentKind = "app_pool_down"
	IncidentSiteDown        IncidentKind = "site_down"
	IncidentWindowsExporter IncidentKind = "windows_exporter_down"
	IncidentFluentBit       IncidentKind = "fluent_bit_down"
	IncidentHealthCheckDown IncidentKind = "health_check_down"
	IncidentHealthCheckSlow IncidentKind = "health_check_slow"
	IncidentHighCPU         IncidentKind = "high_cpu"
	IncidentHighMemory      IncidentKind = "high_memory"
	IncidentLowDisk         IncidentKind = "low_disk"
)

type IncidentRole string

const (
	// RoleOpens starts an incident if one is not already running for that kind and subject.
	RoleOpens IncidentRole = "opens"
	// RoleCloses resolves the matching open incident, and does nothing if there is none -
	// a recovery with no preceding failure is normal after a restart.
	RoleCloses IncidentRole = "closes"
)

type IncidentBinding struct {
	Kind IncidentKind
	Role IncidentRole
}

// IncidentBindings maps the alert types that participate in an incident lifecycle. Types
// absent from this map are informational and never open or close anything: an agent
// updating, a planned shutdown, or a machine being reassigned are all events worth
// recording in the activity feed but none of them is a fault with a duration.
//
// Note machine_shutdown and machine_restarting are deliberately excluded while
// machine_disconnected opens an incident - the difference between a host going away
// because someone asked it to and a host going away on its own is the entire point.
var IncidentBindings = map[AlertType]IncidentBinding{
	AlertMachineDisconnected: {IncidentMachineDown, RoleOpens},
	AlertMachineConnected:    {IncidentMachineDown, RoleCloses},

	AlertAppPoolStopped: {IncidentAppPoolDown, RoleOpens},
	AlertAppPoolStarted: {IncidentAppPoolDown, RoleCloses},

	AlertSiteStopped: {IncidentSiteDown, RoleOpens},
	AlertSiteStarted: {IncidentSiteDown, RoleCloses},

	AlertWindowsExporterDown: {IncidentWindowsExporter, RoleOpens},
	AlertWindowsExporterUp:   {IncidentWindowsExporter, RoleCloses},
	AlertFluentBitDown:       {IncidentFluentBit, RoleOpens},
	AlertFluentBitUp:         {IncidentFluentBit, RoleCloses},

	AlertHealthCheckDown: {IncidentHealthCheckDown, RoleOpens},
	AlertHealthCheckUp:   {IncidentHealthCheckDown, RoleCloses},
	AlertHealthCheckSlow: {IncidentHealthCheckSlow, RoleOpens},
	AlertHealthCheckFast: {IncidentHealthCheckSlow, RoleCloses},

	AlertHighCPU:             {IncidentHighCPU, RoleOpens},
	AlertHighCPURecovered:    {IncidentHighCPU, RoleCloses},
	AlertHighMemory:          {IncidentHighMemory, RoleOpens},
	AlertHighMemoryRecovered: {IncidentHighMemory, RoleCloses},
	AlertLowDisk:             {IncidentLowDisk, RoleOpens},
	AlertLowDiskRecovered:    {IncidentLowDisk, RoleCloses},
}
