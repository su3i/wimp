// Package sqlmetrics evaluates the SQL Server DMV metrics agents publish through
// windows_exporter's textfile collector, and edge-triggers alerts from them. Same shape and
// same tradeoffs as internal/application/metrics: polled from Prometheus, breach state in
// memory, reset on restart.
package sqlmetrics

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/su3i/wimp/internal/application/notification"
	"github.com/su3i/wimp/internal/cache"
	"github.com/su3i/wimp/internal/config"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	notificationDomain "github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/prometheus"
)

const checkInterval = 60 * time.Second

const (
	defaultBackupHours      = 26
	defaultLogBackupMinutes = 120
	defaultBlockingSeconds  = 60
)

type sample struct {
	labels map[string]string
	value  float64
}

// subject names the one thing an alert is about. machineScoped is false for anything that
// outlives a single host: a clustered instance or an availability group, which have to keep
// one incident across a failover.
type subject struct {
	key, display  string
	machineScoped bool
}

type rule struct {
	alert, recovered notificationDomain.AlertType
	query            string
	subject          func(l map[string]string) subject
	// combine folds samples sharing a subject: an AG database is reported by every replica,
	// and its backup is as recent as the newest one taken on any of them.
	combine  func(a, b float64) float64
	breached func(v float64) bool
	describe func(l map[string]string, v float64, breached bool) (event, detail string)
}

type alert struct {
	alertType notificationDomain.AlertType
	projectID uint
	machineID uint
	instance  string
	subject   string
	title     string
	detail    string
	repeat    bool
}

var (
	mu       sync.Mutex
	breaches = map[string]bool{}
)

func StartChecker(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	if prometheusUrl == "" {
		log.Println("sql metrics checker: PROMETHEUSURL not set, SQL alert checking disabled")
		return
	}
	go func() {
		time.Sleep(60 * time.Second) // let Prometheus warm up, same as the other checkers
		for {
			runCheck(dbCfg, prometheusUrl)
			time.Sleep(checkInterval)
		}
	}()
}

func runCheck(dbCfg *config.DatabaseConfig, prometheusUrl string) {
	all, err := database.NewMachineRepository(dbCfg).FindAll()
	if err != nil || all == nil {
		return
	}
	machines := map[string]machineDomain.Machine{}
	for _, m := range *all {
		if !m.Status.Terminal() {
			machines[strconv.Itoa(int(m.ID))] = m
		}
	}

	for _, r := range rules() {
		samples, err := query(prometheusUrl, r.query)
		if err != nil {
			log.Printf("sql metrics checker: query failed (%s): %v", r.alert, err)
			continue
		}
		for _, a := range evaluate(r, samples, machines) {
			if a.repeat {
				notification.EmitRepeatAlert(a.alertType, a.projectID, a.machineID, a.instance, a.subject, a.title, a.detail, dbCfg)
			} else {
				notification.EmitAlert(a.alertType, a.projectID, a.machineID, a.instance, a.subject, a.title, a.detail, dbCfg)
			}
		}
	}
}

func query(prometheusUrl, q string) ([]sample, error) {
	pr, err := prometheus.QueryInstant(prometheusUrl, q)
	if err != nil {
		return nil, err
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("status %s", pr.Status)
	}
	out := make([]sample, 0, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		var s string
		if err := json.Unmarshal(r.Value[1], &s); err != nil {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}
		out = append(out, sample{labels: r.Metric, value: v})
	}
	return out, nil
}

// evaluate groups one rule's samples by subject and returns the alerts their transitions
// produce. A subject missing from this round keeps its state: a database dropped or a job
// deleted while failing leaves its incident for an operator to resolve by hand.
func evaluate(r rule, samples []sample, machines map[string]machineDomain.Machine) []alert {
	type group struct {
		projectID, machineID uint
		subject              subject
		labels               map[string]string
		value                float64
	}
	groups := map[string]*group{}
	order := []string{}
	for _, s := range samples {
		m, ok := machines[s.labels["machine_id"]]
		if !ok {
			continue
		}
		subj := r.subject(s.labels)
		var machineID uint
		if subj.machineScoped {
			machineID = m.ID
		}
		key := fmt.Sprintf("%s|%d:%d:%s", r.alert, m.ProjectID, machineID, subj.key)
		if g, ok := groups[key]; ok {
			g.value = r.combine(g.value, s.value)
			continue
		}
		groups[key] = &group{projectID: m.ProjectID, machineID: machineID, subject: subj, labels: s.labels, value: s.value}
		order = append(order, key)
	}

	mu.Lock()
	defer mu.Unlock()

	var alerts []alert
	for _, key := range order {
		g := groups[key]
		now := r.breached(g.value)
		was := breaches[key]
		mk := func(t notificationDomain.AlertType, breached, repeat bool) alert {
			event, detail := r.describe(g.labels, g.value, breached)
			return alert{t, g.projectID, g.machineID, g.subject.display, g.subject.key,
				notification.AlertTitle(g.subject.display, event), detail, repeat}
		}
		switch {
		case now && !was:
			breaches[key] = true
			alerts = append(alerts, mk(r.alert, true, false))
		case now && was:
			if notification.SeverityFor(r.alert) == notificationDomain.LevelSev && cache.SevRepeatDue(key) {
				alerts = append(alerts, mk(r.alert, true, true))
			}
		case !now && was:
			delete(breaches, key)
			cache.ClearSevRepeat(key)
			alerts = append(alerts, mk(r.recovered, false, false))
		}
	}
	return alerts
}

func serverSubject(l map[string]string, suffix string) subject {
	key := l["sql_server"]
	if suffix != "" {
		key += "/" + suffix
	}
	return subject{key: key, display: l["sql_server"], machineScoped: l["clustered"] != "true"}
}

// databaseSubject collapses an availability group database across its replicas.
func databaseSubject(l map[string]string) subject {
	if ag := l["availability_group"]; ag != "" {
		return subject{key: "ag:" + ag + "/" + l["database"], display: ag}
	}
	return serverSubject(l, l["database"])
}

func minOf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxOf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

var databaseStates = map[float64]string{3: "RECOVERY_PENDING", 4: "SUSPECT", 5: "EMERGENCY"}

func fmtAge(seconds float64) string {
	switch {
	case seconds >= 48*3600:
		return fmt.Sprintf("%.1f days", seconds/86400)
	case seconds >= 3600:
		return fmt.Sprintf("%.1f hours", seconds/3600)
	default:
		return fmt.Sprintf("%.0f minutes", seconds/60)
	}
}

func thresholdOr(configured, fallback int) float64 {
	if configured > 0 {
		return float64(configured)
	}
	return float64(fallback)
}

func rules() []rule {
	a := config.Alerts()
	backupSec := thresholdOr(a.ThresholdSqlBackupHours, defaultBackupHours) * 3600
	logBackupSec := thresholdOr(a.ThresholdSqlLogBackupMinutes, defaultLogBackupMinutes) * 60
	blockingSec := thresholdOr(a.ThresholdSqlBlockingSeconds, defaultBlockingSeconds)

	// Backups are only expected of databases that are ONLINE: a RESTORING database (log
	// shipping secondary, a restore in progress) is not supposed to be backed up.
	onlineOnly := ` and on (machine_id, sql_instance, database) (wimp_mssql_database_state == 0)`

	backup := func(kind, label string, threshold float64, alertType, recovered notificationDomain.AlertType) rule {
		return rule{
			alert: alertType, recovered: recovered,
			query:    `wimp_mssql_database_backup_age_seconds{kind="` + kind + `"}` + onlineOnly,
			subject:  databaseSubject,
			combine:  minOf,
			breached: func(v float64) bool { return v > threshold },
			describe: func(l map[string]string, v float64, breached bool) (string, string) {
				if breached {
					return label + " Overdue: " + l["database"], fmt.Sprintf("No %s of %s on %s for %s (threshold %s)",
						lowerFirst(label), l["database"], l["sql_server"], fmtAge(v), fmtAge(threshold))
				}
				return label + " Recovered: " + l["database"], fmt.Sprintf("%s of %s on %s is current again (%s old)",
					label, l["database"], l["sql_server"], fmtAge(v))
			},
		}
	}

	return []rule{
		{
			alert: notificationDomain.AlertSqlDatabaseUnhealthy, recovered: notificationDomain.AlertSqlDatabaseRecovered,
			query:   `wimp_mssql_database_state`,
			subject: func(l map[string]string) subject { return serverSubject(l, l["database"]) },
			combine: maxOf,
			// OFFLINE is excluded: taking a database offline is something an operator does.
			breached: func(v float64) bool { _, bad := databaseStates[v]; return bad },
			describe: func(l map[string]string, v float64, breached bool) (string, string) {
				if breached {
					return "Database " + databaseStates[v] + ": " + l["database"], fmt.Sprintf("Database %s on %s is %s and cannot be used", l["database"], l["sql_server"], databaseStates[v])
				}
				return "Database Recovered: " + l["database"], fmt.Sprintf("Database %s on %s has left its unhealthy state", l["database"], l["sql_server"])
			},
		},
		backup("full", "Backup", backupSec, notificationDomain.AlertSqlBackupOverdue, notificationDomain.AlertSqlBackupRecovered),
		backup("log", "Log Backup", logBackupSec, notificationDomain.AlertSqlLogBackupOverdue, notificationDomain.AlertSqlLogBackupRecovered),
		{
			alert: notificationDomain.AlertSqlJobFailed, recovered: notificationDomain.AlertSqlJobRecovered,
			query:    `wimp_mssql_job_last_run_failed`,
			subject:  func(l map[string]string) subject { return serverSubject(l, "job:"+l["job"]) },
			combine:  maxOf,
			breached: func(v float64) bool { return v == 1 },
			describe: func(l map[string]string, _ float64, breached bool) (string, string) {
				if breached {
					return "Job Failed: " + l["job"], fmt.Sprintf("SQL Agent job %s on %s failed its last run", l["job"], l["sql_server"])
				}
				return "Job Recovered: " + l["job"], fmt.Sprintf("SQL Agent job %s on %s succeeded", l["job"], l["sql_server"])
			},
		},
		{
			alert: notificationDomain.AlertSqlAgUnhealthy, recovered: notificationDomain.AlertSqlAgHealthy,
			query: `wimp_mssql_ag_replica_healthy`,
			subject: func(l map[string]string) subject {
				return subject{key: "ag:" + l["availability_group"] + "/" + l["replica"], display: l["availability_group"]}
			},
			// Unhealthy if any replica's view says so; the primary is the one that sees them all.
			combine:  minOf,
			breached: func(v float64) bool { return v == 0 },
			describe: func(l map[string]string, _ float64, breached bool) (string, string) {
				if breached {
					return "Replica Unhealthy: " + l["replica"], fmt.Sprintf("Replica %s of availability group %s is not synchronizing healthily", l["replica"], l["availability_group"])
				}
				return "Replica Healthy: " + l["replica"], fmt.Sprintf("Replica %s of availability group %s is healthy again", l["replica"], l["availability_group"])
			},
		},
		{
			alert: notificationDomain.AlertSqlBlocking, recovered: notificationDomain.AlertSqlBlockingCleared,
			query:    `wimp_mssql_longest_block_seconds`,
			subject:  func(l map[string]string) subject { return serverSubject(l, "") },
			combine:  maxOf,
			breached: func(v float64) bool { return v > blockingSec },
			describe: func(l map[string]string, v float64, breached bool) (string, string) {
				if breached {
					return "Blocking", fmt.Sprintf("A request on %s has been blocked for %.0fs (threshold %.0fs)", l["sql_server"], v, blockingSec)
				}
				return "Blocking Cleared", fmt.Sprintf("Blocking on %s has cleared", l["sql_server"])
			},
		},
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]+('a'-'A')) + s[1:]
}
