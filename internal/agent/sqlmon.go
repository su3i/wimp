package agent

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/su3i/wimp/internal/domain/protocol"
)

const (
	sqlPollInterval = 60 * time.Second
	sqlQueryTimeout = 20 * time.Second
	// The textfile collector reads *.prom only, so the temp name must not end in .prom.
	sqlTextfileName = "wimp_mssql.prom"
)

// sqlMonitor polls DMVs on the SQL instances this node is responsible for and writes the
// results as a Prometheus text file, which windows_exporter's textfile collector serves on
// the scrape Prometheus already makes. No new port, no new scrape config.
type sqlMonitor struct {
	mu        sync.Mutex
	cfg       *protocol.SqlMonitorConfigPayload // nil until the control plane sends one
	instances []protocol.SqlInstanceInfo        // latest heartbeat snapshot
	errs      map[string]string                 // instance -> last poll error
	once      sync.Once
}

func newSqlMonitor() *sqlMonitor {
	return &sqlMonitor{errs: map[string]string{}}
}

func (m *sqlMonitor) setConfig(cfg protocol.SqlMonitorConfigPayload) {
	m.mu.Lock()
	m.cfg = &cfg
	m.mu.Unlock()
}

// observe records the latest instance snapshot and annotates it with monitor status.
func (m *sqlMonitor) observe(instances []protocol.SqlInstanceInfo, hostname string) []protocol.SqlInstanceInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances = instances
	for i := range instances {
		if shouldMonitor(instances[i], hostname) {
			instances[i].Monitored = true
			instances[i].MonitorError = m.errs[instances[i].Name]
		}
	}
	return instances
}

// start runs the poll loop for the agent's lifetime. Safe to call more than once.
func (m *sqlMonitor) start(ctx context.Context, textfileDir, hostname string) {
	m.once.Do(func() {
		go func() {
			for {
				m.poll(ctx, textfileDir, hostname)
				select {
				case <-ctx.Done():
					return
				case <-time.After(sqlPollInterval):
				}
			}
		}()
	})
}

// shouldMonitor is true on exactly one node per instance: a standalone instance where it is
// running, a clustered one on the node that owns it.
func shouldMonitor(inst protocol.SqlInstanceInfo, hostname string) bool {
	if inst.State != protocol.SqlStateRunning {
		return false
	}
	return !inst.Clustered || strings.EqualFold(inst.OwnerNode, hostname)
}

func (m *sqlMonitor) poll(ctx context.Context, textfileDir, hostname string) {
	m.mu.Lock()
	cfg, instances := m.cfg, m.instances
	m.mu.Unlock()
	if cfg == nil {
		return
	}

	errs := map[string]string{}
	w := newPromWriter()
	for _, inst := range instances {
		if !shouldMonitor(inst, hostname) {
			continue
		}
		if textfileDir == "" {
			errs[inst.Name] = "metrics directory not configured on this host, re-run the bootstrap script"
			continue
		}
		if err := collectInstance(ctx, w, inst, *cfg, hostname); err != nil {
			errs[inst.Name] = err.Error()
		}
	}

	m.mu.Lock()
	m.errs = errs
	m.mu.Unlock()

	if textfileDir != "" {
		writeAtomic(filepath.Join(textfileDir, sqlTextfileName), w.String()) //nolint:errcheck
	}
}

// query runs one live query on an instance this node monitors.
func (m *sqlMonitor) query(ctx context.Context, instance, kind, hostname string) ([]map[string]any, error) {
	m.mu.Lock()
	cfg, instances := m.cfg, m.instances
	m.mu.Unlock()
	if cfg == nil {
		return nil, fmt.Errorf("SQL monitoring is not configured yet")
	}
	stmt, ok := liveQueries[kind]
	if !ok {
		return nil, fmt.Errorf("unknown query kind %q", kind)
	}
	for _, inst := range instances {
		if inst.Name != instance {
			continue
		}
		if !shouldMonitor(inst, hostname) {
			return nil, fmt.Errorf("instance %s is not running on this node", instance)
		}
		db, err := openInstance(inst, *cfg)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		ctx, cancel := context.WithTimeout(ctx, sqlQueryTimeout)
		defer cancel()
		return queryRows(ctx, db, stmt)
	}
	return nil, fmt.Errorf("instance %s not found", instance)
}

func openInstance(inst protocol.SqlInstanceInfo, cfg protocol.SqlMonitorConfigPayload) (*sql.DB, error) {
	return sql.Open("sqlserver", connString(inst, cfg))
}

// connString targets the local instance. An FCI only listens on its virtual network name,
// never on the node's own addresses, so it is reached through that name.
func connString(inst protocol.SqlInstanceInfo, cfg protocol.SqlMonitorConfigPayload) string {
	host := "localhost"
	if inst.Clustered && inst.VirtualServerName != "" {
		host = inst.VirtualServerName
	}
	u := &url.URL{Scheme: "sqlserver", Host: host}
	if port, _, _ := strings.Cut(inst.Port, ","); port != "" && port != "0" {
		u.Host = host + ":" + port
	} else if !strings.EqualFold(inst.Name, "MSSQLSERVER") {
		u.Path = inst.Name // resolved through SQL Browser
	}
	if cfg.Username != "" {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}
	// No user means the driver authenticates with Windows SSPI as the agent's service account.
	// TrustServerCertificate: the connection never leaves this host or its own FCI name, and
	// instances commonly run on their self-signed certificate.
	q := url.Values{}
	q.Set("database", "master")
	q.Set("app name", "wimp-agent")
	q.Set("dial timeout", "10")
	q.Set("TrustServerCertificate", "true")
	u.RawQuery = q.Encode()
	return u.String()
}

func collectInstance(ctx context.Context, w *promWriter, inst protocol.SqlInstanceInfo, cfg protocol.SqlMonitorConfigPayload, hostname string) error {
	server := hostname
	if inst.Clustered && inst.VirtualServerName != "" {
		server = inst.VirtualServerName
	}
	if !strings.EqualFold(inst.Name, "MSSQLSERVER") {
		server += `\` + inst.Name
	}
	base := []label{{"sql_instance", inst.Name}, {"sql_server", strings.ToUpper(server)}, {"clustered", fmt.Sprint(inst.Clustered)}}

	ctx, cancel := context.WithTimeout(ctx, sqlQueryTimeout)
	defer cancel()

	db, err := openInstance(inst, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		w.add("wimp_mssql_up", "gauge", "Whether the agent could connect and authenticate to the instance.", base, 0)
		return err
	}
	w.add("wimp_mssql_up", "gauge", "Whether the agent could connect and authenticate to the instance.", base, 1)

	// Each collector is independent: a login lacking msdb access still gets wait stats. The
	// first failure is reported, since it is usually the one permission the DBA has to add.
	var firstErr error
	for _, collect := range []func(context.Context, *sql.DB, *promWriter, []label) error{
		collectWaits, collectRequests, collectDatabases, collectJobs, collectAvailabilityGroups,
	} {
		if err := collect(ctx, db, w, base); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Waits that are idle background loops rather than anything a workload waits on. Leaving
// them in buries real waits under ones that grow forever on a healthy server.
const benignWaits = `
'BROKER_EVENTHANDLER','BROKER_RECEIVE_WAITFOR','BROKER_TASK_STOP','BROKER_TO_FLUSH','BROKER_TRANSMITTER',
'CHECKPOINT_QUEUE','CHKPT','CLR_AUTO_EVENT','CLR_MANUAL_EVENT','CLR_SEMAPHORE','DBMIRROR_DBM_EVENT',
'DBMIRROR_EVENTS_QUEUE','DBMIRROR_WORKER_QUEUE','DBMIRRORING_CMD','DIRTY_PAGE_POLL','DISPATCHER_QUEUE_SEMAPHORE',
'EXECSYNC','FSAGENT','FT_IFTS_SCHEDULER_IDLE_WAIT','FT_IFTSHC_MUTEX','HADR_CLUSAPI_CALL',
'HADR_FILESTREAM_IOMGR_IOCOMPLETION','HADR_LOGCAPTURE_WAIT','HADR_NOTIFICATION_DEQUEUE','HADR_TIMER_TASK',
'HADR_WORK_QUEUE','KSOURCE_WAKEUP','LAZYWRITER_SLEEP','LOGMGR_QUEUE','MEMORY_ALLOCATION_EXT',
'ONDEMAND_TASK_QUEUE','PARALLEL_REDO_DRAIN_WORKER','PARALLEL_REDO_LOG_CACHE','PARALLEL_REDO_TRAN_LIST',
'PARALLEL_REDO_WORKER_SYNC','PARALLEL_REDO_WORKER_WAIT_WORK','PREEMPTIVE_OS_FLUSHFILEBUFFERS',
'PREEMPTIVE_XE_GETTARGETSTATE','PVS_PREALLOCATE','PWAIT_ALL_COMPONENTS_INITIALIZED',
'PWAIT_DIRECTLOGCONSUMER_GETNEXT','PWAIT_EXTENSIBILITY_CLEANUP_TASK','QDS_ASYNC_QUEUE',
'QDS_CLEANUP_STALE_QUERIES_TASK_MAIN_LOOP_SLEEP','QDS_PERSIST_TASK_MAIN_LOOP_SLEEP','QDS_SHUTDOWN_QUEUE',
'REDO_THREAD_PENDING_WORK','REQUEST_FOR_DEADLOCK_SEARCH','RESOURCE_QUEUE','SERVER_IDLE_CHECK',
'SLEEP_BPOOL_FLUSH','SLEEP_DBSTARTUP','SLEEP_DCOMSTARTUP','SLEEP_MASTERDBREADY','SLEEP_MASTERMDREADY',
'SLEEP_MASTERUPGRADED','SLEEP_MSDBSTARTUP','SLEEP_SYSTEMTASK','SLEEP_TASK','SLEEP_TEMPDBSTARTUP',
'SNI_HTTP_ACCEPT','SOS_WORK_DISPATCHER','SP_SERVER_DIAGNOSTICS_SLEEP','SQLTRACE_BUFFER_FLUSH',
'SQLTRACE_INCREMENTAL_FLUSH_SLEEP','SQLTRACE_WAIT_ENTRIES','UCS_SESSION_REGISTRATION','VDI_CLIENT_OTHER',
'WAIT_FOR_RESULTS','WAIT_XTP_CKPT_CLOSE','WAIT_XTP_HOST_WAIT','WAIT_XTP_OFFLINE_CKPT_NEW_LOG',
'WAIT_XTP_RECOVERY','WAITFOR','WAITFOR_TASKSHUTDOWN','XE_BUFFERMGR_ALLPROCESSED_EVENT','XE_DISPATCHER_JOIN',
'XE_DISPATCHER_WAIT','XE_LIVE_TARGET_TVF','XE_TIMER_EVENT'`

func collectWaits(ctx context.Context, db *sql.DB, w *promWriter, base []label) error {
	rows, err := db.QueryContext(ctx, `SELECT wait_type, wait_time_ms, waiting_tasks_count FROM sys.dm_os_wait_stats
		WHERE wait_time_ms > 0 AND wait_type NOT IN (`+benignWaits+`)`)
	if err != nil {
		return fmt.Errorf("wait stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var waitType string
		var waitMs, tasks int64
		if err := rows.Scan(&waitType, &waitMs, &tasks); err != nil {
			return err
		}
		l := with(base, label{"wait_type", waitType})
		w.add("wimp_mssql_wait_seconds_total", "counter", "Time spent waiting, by wait type, since the instance started.", l, float64(waitMs)/1000)
		w.add("wimp_mssql_waiting_tasks_total", "counter", "Waits that occurred, by wait type, since the instance started.", l, float64(tasks))
	}
	return rows.Err()
}

func collectRequests(ctx context.Context, db *sql.DB, w *promWriter, base []label) error {
	var blocked, longestBlockMs, longestRequestSec int64
	err := db.QueryRowContext(ctx, `SELECT
			COUNT(CASE WHEN r.blocking_session_id > 0 THEN 1 END),
			ISNULL(MAX(CASE WHEN r.blocking_session_id > 0 THEN r.wait_time END), 0),
			ISNULL(MAX(CASE WHEN r.wait_type NOT IN ('WAITFOR', 'BROKER_RECEIVE_WAITFOR') OR r.wait_type IS NULL
				THEN r.total_elapsed_time / 1000 END), 0)
		FROM sys.dm_exec_requests r
		JOIN sys.dm_exec_sessions s ON s.session_id = r.session_id
		WHERE s.is_user_process = 1 AND r.session_id <> @@SPID`).Scan(&blocked, &longestBlockMs, &longestRequestSec)
	if err != nil {
		return fmt.Errorf("requests: %w", err)
	}
	w.add("wimp_mssql_blocked_requests", "gauge", "User requests currently blocked by another session.", base, float64(blocked))
	w.add("wimp_mssql_longest_block_seconds", "gauge", "How long the longest-blocked request has been waiting.", base, float64(longestBlockMs)/1000)
	w.add("wimp_mssql_longest_request_seconds", "gauge", "Elapsed time of the longest-running user request.", base, float64(longestRequestSec))
	return nil
}

// collectDatabases reports state and backup age per database. Ages are computed on the server
// against its own clock, since msdb stores backup times in server local time. A database
// never backed up counts from its creation, so it alerts rather than hiding.
func collectDatabases(ctx context.Context, db *sql.DB, w *promWriter, base []label) error {
	rows, err := db.QueryContext(ctx, `SELECT d.name, d.state, d.recovery_model_desc, ISNULL(ag.name, ''),
			DATEDIFF(SECOND, ISNULL((SELECT MAX(b.backup_finish_date) FROM msdb.dbo.backupset b
				WHERE b.database_name = d.name AND b.type IN ('D', 'I')), d.create_date), GETDATE()),
			DATEDIFF(SECOND, ISNULL((SELECT MAX(b.backup_finish_date) FROM msdb.dbo.backupset b
				WHERE b.database_name = d.name AND b.type = 'L'), d.create_date), GETDATE())
		FROM sys.databases d
		LEFT JOIN sys.availability_replicas ar ON ar.replica_id = d.replica_id
		LEFT JOIN sys.availability_groups ag ON ag.group_id = ar.group_id
		WHERE d.name NOT IN ('tempdb', 'model') AND d.source_database_id IS NULL`)
	if err != nil {
		return fmt.Errorf("databases: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, recovery, ag string
		var state int
		var fullAge, logAge int64
		if err := rows.Scan(&name, &state, &recovery, &ag, &fullAge, &logAge); err != nil {
			return err
		}
		l := with(base, label{"database", name}, label{"availability_group", ag})
		w.add("wimp_mssql_database_state", "gauge", "sys.databases state: 0 ONLINE, 1 RESTORING, 2 RECOVERING, 3 RECOVERY_PENDING, 4 SUSPECT, 5 EMERGENCY, 6 OFFLINE.", l, float64(state))
		w.add("wimp_mssql_database_backup_age_seconds", "gauge", "Seconds since the last backup recorded in this instance's msdb, by kind.", with(l, label{"kind", "full"}), float64(fullAge))
		// Log backups only exist for databases that keep their log, so only those report one.
		if recovery != "SIMPLE" {
			w.add("wimp_mssql_database_backup_age_seconds", "gauge", "Seconds since the last backup recorded in this instance's msdb, by kind.", with(l, label{"kind", "log"}), float64(logAge))
		}
	}
	return rows.Err()
}

func collectJobs(ctx context.Context, db *sql.DB, w *promWriter, base []label) error {
	rows, err := db.QueryContext(ctx, `SELECT j.name, ISNULL(h.run_status, -1)
		FROM msdb.dbo.sysjobs j
		OUTER APPLY (SELECT TOP 1 run_status FROM msdb.dbo.sysjobhistory h
			WHERE h.job_id = j.job_id AND h.step_id = 0 ORDER BY h.instance_id DESC) h
		WHERE j.enabled = 1`)
	if err != nil {
		return fmt.Errorf("agent jobs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var status int
		if err := rows.Scan(&name, &status); err != nil {
			return err
		}
		failed := 0.0
		if status == 0 {
			failed = 1
		}
		w.add("wimp_mssql_job_last_run_failed", "gauge", "1 when an enabled SQL Agent job's last completed run failed.", with(base, label{"job", name}), failed)
	}
	return rows.Err()
}

func collectAvailabilityGroups(ctx context.Context, db *sql.DB, w *promWriter, base []label) error {
	var hadr int
	if err := db.QueryRowContext(ctx, `SELECT ISNULL(CAST(SERVERPROPERTY('IsHadrEnabled') AS int), 0)`).Scan(&hadr); err != nil || hadr != 1 {
		return nil
	}
	// The primary sees every replica; a secondary sees only itself.
	rows, err := db.QueryContext(ctx, `SELECT ag.name, ar.replica_server_name, ISNULL(rs.role_desc, ''), ISNULL(rs.synchronization_health, 0)
		FROM sys.dm_hadr_availability_replica_states rs
		JOIN sys.availability_replicas ar ON ar.replica_id = rs.replica_id
		JOIN sys.availability_groups ag ON ag.group_id = rs.group_id`)
	if err != nil {
		return fmt.Errorf("availability groups: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ag, replica, role string
		var health int
		if err := rows.Scan(&ag, &replica, &role, &health); err != nil {
			return err
		}
		healthy := 0.0
		if health == 2 {
			healthy = 1
		}
		w.add("wimp_mssql_ag_replica_healthy", "gauge", "1 when an availability group replica reports HEALTHY synchronization.",
			with(base, label{"availability_group", ag}, label{"replica", replica}, label{"role", role}), healthy)
	}
	return rows.Err()
}

var liveQueries = map[string]string{
	protocol.SqlQueryBlocking: `SELECT r.session_id, r.blocking_session_id, r.wait_type, r.wait_time AS wait_ms,
			DB_NAME(r.database_id) AS database_name, s.login_name, s.host_name, s.program_name,
			bs.login_name AS blocker_login, bs.host_name AS blocker_host, bs.program_name AS blocker_program,
			bs.status AS blocker_status, LEFT(t.text, 4000) AS sql_text
		FROM sys.dm_exec_requests r
		JOIN sys.dm_exec_sessions s ON s.session_id = r.session_id
		LEFT JOIN sys.dm_exec_sessions bs ON bs.session_id = r.blocking_session_id
		OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) t
		WHERE r.blocking_session_id > 0
		ORDER BY r.wait_time DESC`,
	protocol.SqlQueryRequests: `SELECT TOP 50 r.session_id, r.status, r.command, r.wait_type, r.wait_time AS wait_ms,
			r.total_elapsed_time AS elapsed_ms, r.cpu_time AS cpu_ms, r.logical_reads,
			DB_NAME(r.database_id) AS database_name, s.login_name, s.host_name, s.program_name,
			LEFT(t.text, 4000) AS sql_text
		FROM sys.dm_exec_requests r
		JOIN sys.dm_exec_sessions s ON s.session_id = r.session_id
		OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) t
		WHERE s.is_user_process = 1 AND r.session_id <> @@SPID
		ORDER BY r.total_elapsed_time DESC`,
	protocol.SqlQueryJobs: `SELECT j.name, j.enabled, c.name AS category, h.run_status AS last_run_status,
			CASE WHEN h.run_date > 0 THEN DATETIMEFROMPARTS(h.run_date / 10000, h.run_date / 100 % 100, h.run_date % 100,
				h.run_time / 10000, h.run_time / 100 % 100, h.run_time % 100, 0) END AS last_run_at,
			h.run_duration AS last_run_duration_hhmmss, LEFT(h.message, 1000) AS last_message,
			a.next_scheduled_run_date AS next_run_at
		FROM msdb.dbo.sysjobs j
		LEFT JOIN msdb.dbo.syscategories c ON c.category_id = j.category_id
		OUTER APPLY (SELECT TOP 1 run_status, run_date, run_time, run_duration, message FROM msdb.dbo.sysjobhistory h
			WHERE h.job_id = j.job_id AND h.step_id = 0 ORDER BY h.instance_id DESC) h
		OUTER APPLY (SELECT TOP 1 next_scheduled_run_date FROM msdb.dbo.sysjobactivity a
			WHERE a.job_id = j.job_id ORDER BY a.session_id DESC) a
		ORDER BY j.name`,
}

func queryRows(ctx context.Context, db *sql.DB, stmt string) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ── Prometheus text format ─────────────────────────────────────────────────────

type label struct{ name, value string }

func with(base []label, extra ...label) []label {
	return append(append([]label{}, base...), extra...)
}

type promFamily struct {
	help, typ string
	lines     []string
}

type promWriter struct {
	families map[string]*promFamily
}

func newPromWriter() *promWriter {
	return &promWriter{families: map[string]*promFamily{}}
}

func (w *promWriter) add(name, typ, help string, labels []label, value float64) {
	f, ok := w.families[name]
	if !ok {
		f = &promFamily{help: help, typ: typ}
		w.families[name] = f
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		parts[i] = fmt.Sprintf(`%s="%s"`, l.name, escapeLabel(l.value))
	}
	f.lines = append(f.lines, fmt.Sprintf("%s{%s} %g", name, strings.Join(parts, ","), value))
}

func (w *promWriter) String() string {
	names := make([]string, 0, len(w.families))
	for n := range w.families {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		f := w.families[n]
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", n, f.help, n, f.typ)
		for _, l := range f.lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func escapeLabel(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}

// writeAtomic replaces path in one step, so the collector never reads a half-written file.
func writeAtomic(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
