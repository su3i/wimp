# Architecture

## System at a glance

```
                        Windows host (one per server)
 ┌──────────────────────────────────────────────────────────────────┐
 │  wimp-agent (Go, Windows service, runs as LocalSystem)           │
 │    - dials OUT to control plane over WebSocket                   │
 │    - discovers IIS / SQL / cluster, sends heartbeat every 30 s   │
 │    - executes commands, writes SQL DMV metrics as .prom file     │
 │                                                                  │
 │  windows_exporter :9182  <── scraped by Prometheus (inbound)     │
 │    collectors chosen at bootstrap: cpu, memory, disk, net, iis,  │
 │    mssql, mscluster, textfile (reads the agent's .prom file)     │
 │                                                                  │
 │  fluent-bit  ──> pushes IIS/app log files + Event Log to Loki    │
 └──────────────────────────────────────────────────────────────────┘
        │ wss://APPURL/ws/agent?token=…                ▲ scrape :9182
        ▼                                              │
 ┌───────────────────────────── Kubernetes (one Helm release) ───────────────────────────┐
 │                                                                                        │
 │  wimp-control-plane (Go/Gin, 1 replica)                                                │
 │    REST API for the browser, agent WS hub, browser WS broadcast,                      │
 │    Prometheus HTTP service discovery, 3 background checkers that query Prometheus,    │
 │    alert pipeline -> Alertmanager                                                      │
 │        │                       │                       │                               │
 │        ▼                       ▼                       ▼                               │
 │   Postgres or SQLite      Prometheus  <── blackbox exporter (HTTP health checks)       │
 │   (state of record)       (metrics, SD from control plane)                             │
 │                                                                                        │
 │   Loki + MinIO (logs)      Alertmanager ──> Telegram / Slack / whatever receiver       │
 │                                                                                        │
 │  wimp-web (nginx serving React SPA)                                                    │
 └────────────────────────────────────────────────────────────────────────────────────────┘
        ▲  REST + WS to control plane, PromQL straight to Prometheus
        │
     Browser
```

## Components

| Component | Code | Runtime | Responsibility |
|---|---|---|---|
| Agent | `cmd/agent`, `internal/agent` | Windows service `wimp-agent`, LocalSystem | Connect out, register, discover, heartbeat, execute commands, reconcile Fluent Bit config, poll SQL DMVs, self-update |
| windows_exporter | pinned 0.31.7, installed by `data/bootstrap.ps1` | Windows service | Host metrics on :9182. `textfile` collector serves the agent's SQL metrics file. |
| Fluent Bit | pinned 5.0.7, installed by bootstrap | Windows service | Tails log directories configured per app pool, plus Event Log Application/System channels, ships to Loki |
| Control plane | `cmd/app`, `internal/*` except `agent` | Container `darksueii/wimp`, Ubuntu 22.04 base | Everything server-side. Single binary, single replica. |
| Web | `web/` | Container `darksueii/wimp-web`, nginx 1.27 | Static SPA. API URL and Prometheus URL substituted into the JS at container start. |
| Database | `internal/infrastructure/database` | Postgres (Helm default) or SQLite (local default) | State of record: projects, accounts, machines, pools, sites, SQL instances, applications, notifications, incidents, settings |
| Cache | `internal/infrastructure/cache` | In-memory (default) or Redis | Refresh tokens, MFA challenges, rate-limit counters. Helm does not set `CACHETYPE`, so production uses memory. |
| Prometheus | kube-prometheus-stack subchart | | Scrapes windows_exporter via HTTP SD from `/prometheus/targets`, runs blackbox probes via `/prometheus/monitors` |
| Blackbox exporter | prometheus-blackbox-exporter subchart | | `http_2xx` probes for application health checks |
| Loki | loki subchart, monolithic, MinIO backend | | Log store. 7-day retention, `deletion_mode: filter-and-delete` so "clear logs" works. |
| Alertmanager | part of kube-prometheus-stack | | Only outbound alert channel. Routes `source="wimp"` to a receiver. |

## Network directions

| From | To | Port/path | Why it matters |
|---|---|---|---|
| Agent | Control plane | `GET /ws/agent?token=` (WebSocket) | The only control channel. Outbound from the host. |
| Agent | GitHub releases | HTTPS | Downloads `agent.exe` on self-update |
| Fluent Bit | Loki | 3100 | Log push. Host must reach Loki. |
| Prometheus | Host | 9182 | Scrape is inbound to the host. Bootstrap adds the firewall exception. So "nothing inbound" is true for control, not for metrics. |
| Prometheus | Control plane | `/prometheus/targets`, `/prometheus/monitors` | HTTP SD. Unauthenticated. Hardcoded in values as `http://wimp-control-plane/...` (assumes release name `wimp`). |
| Blackbox exporter | Application URLs | HTTP(S) | Health checks run from the cluster, not from hosts |
| Control plane | Prometheus | `PROMETHEUSURL` | Checkers' instant queries |
| Control plane | Loki | `LOKIHOST:LOKIPORT` | Log queries and delete requests |
| Control plane | Alertmanager | `POST /api/v2/alerts` | Alert delivery |
| Browser | Control plane | REST + `GET /ws?token=` | API and live notifications |
| Browser | Prometheus | `/api/v1/query`, `/api/v1/query_range` | Charts. Prometheus must be reachable from the user's browser, and it has no auth in front of it. |

## Process startup (`cmd/app/main.go`)

1. Load `.env` if present (missing is a warning, not fatal), then `envconfig` into five config structs. Missing required vars are fatal.
2. Connect DB and run GORM `AutoMigrate` for every model.
3. Load the bootstrap token into metadata, then load operator setting overrides into memory before any request or alert can read them.
4. Seed: `Administrator` SUPERADMIN account from `DEFAULTADMIN*`, `Default` organization, `default` project. Each logs and continues if it already exists.
5. Load Casbin enforcer from `./data/model.conf` and `./data/policy.csv`.
6. Start three background checkers (health checks, host metrics, SQL metrics). Each sleeps 60 s first to let Prometheus warm up. Each is disabled if `PROMETHEUSURL` is empty.
7. Build the Gin router and serve. SIGTERM triggers a 10 s graceful shutdown.

## Flow 1: onboarding a host

1. Operator clicks "Add host". `POST /projects/:key/machines` creates a `machines` row with status `pending` and a random 32-byte token valid for 100 years. Response contains two commands:
   `iwr "APPURL/bootstrap?token=…" -OutFile bootstrap.ps1` and `powershell -ExecutionPolicy Bypass -File bootstrap.ps1`.
2. `GET /bootstrap?token=` (public, the token is the credential) renders `data/bootstrap.ps1` as a Go `text/template` with control plane URL, agent download URL (`github.com/su3i/wimp/releases/download/v<AGENTVERSION>/agent.exe`), token, machine id, Loki host/port.
3. On the host, as Administrator, the script:
   - Detects IIS (`appcmd.exe`), SQL Server (registry), cluster (`ClusSvc` running) and picks windows_exporter collectors accordingly. Uninstalls any existing windows_exporter first, because reinstalling the same MSI version silently keeps the old collector list.
   - Installs windows_exporter with the textfile directory and a firewall exception.
   - Installs Fluent Bit with a base config shipping Event Log to Loki.
   - Downloads `agent.exe`, writes `config.json` next to it (URL, token, machine id, directories), installs `wimp-agent` service.
   - Starts all three services, verifies they run, and if windows_exporter failed, digs its error out of the Event Log.
4. The agent connects. The row flips to `online`.

Workloads installed after bootstrap need bootstrap re-run to get their collector.

## Flow 2: the agent connection

Agent side (`internal/agent/connection.go`):

- `connect` loops forever with exponential backoff 1 s to 30 s, reset once a connection has stayed up for 1 minute.
- `dial` opens the socket, sends `register`, sends an immediate heartbeat, starts the 30 s heartbeat goroutine, then runs the read loop.
- All writes go through `safeConn` (a mutex) because heartbeat and command goroutines write concurrently. gorilla/websocket allows one concurrent writer.

Control plane side (`internal/infrastructure/server/handlers/ws.go`, `AgentWebSocket`):

- Look up machine by token. Reject if missing or expired. Upgrade. Set a 90 s read deadline, refreshed on every message.
- Mark online, register the connection in `hub.Get()` keyed by machine id, emit `machine_connected`.
- Read loop dispatches on `msg.Type`:
  - `register`: update identity columns with a scoped update (not a full save), alert `agent_updated` if the version changed, run `ReconcileReassignment`, send `register_ack`, push Fluent Bit config, push SQL monitor credentials, and push an agent update if `AUTOUPDATEAGENT` is on and the version differs.
  - `discovery`: upsert app pools, sites and SQL instances; store cluster membership.
  - `heartbeat`: save `last_seen_at`, diff app pool and site running sets into started/stopped alerts, diff sidecar health against the previous heartbeat, update SQL states and evaluate SQL/cluster edges.
  - `*_result`: resolve the pending command channel by id.
- On disconnect (deferred): deregister, re-read the row's status, then:
  - `deleting`: hard delete the machine and its pools and SQL rows.
  - `reassigned`: silent.
  - A restart was requested in the last 15 s: emit `machine_restarting` (Info, opens no incident).
  - Otherwise: mark offline and emit `machine_disconnected` (Sev by registry default).

The register ack is sent through the hub, not straight to `conn`, because HTTP handlers may be writing to the same socket.

## Flow 3: discovery vs heartbeat

Discovery is sent once per connection, after `register_ack`, and is authoritative: it carries full detail (runtime version, pipeline mode, identity, bindings, physical paths) and is the only thing allowed to clear cluster membership. The heartbeat is cheap: names of running pools and sites, plus the full SQL/cluster snapshot (SQL state has more than two values, so names are not enough). A heartbeat without cluster info is treated as "cluster query failed", not "left the cluster".

On the agent, IIS data comes from `appcmd.exe list … /xml`. SQL instances come from the registry (works when the instance is stopped and needs no credentials). Clustered state comes from one PowerShell call to the FailoverClusters module per heartbeat (a known cost, marked `ponytail:`).

## Flow 4: command round-trip

Every control action (app pool, site, machine restart, SQL action, agent update, list files, download logs, clear logs, SQL live query) uses the same pattern:

1. HTTP handler authorises, verifies the machine belongs to the project, checks `hub.Get().IsOnline`.
2. Generates a UUID, `hub.RegisterCommand(id)` returns a buffered channel.
3. Sends the JSON frame through the hub (per-connection write mutex).
4. For pool/site actions, writes a "pending" state into an in-memory override (`starting`, `stopping`…) with a 30 s TTL, so list views reflect the action before the next heartbeat confirms it.
5. Blocks on the channel with a 30 s timeout (120 s for log downloads). The agent runs the command in its own goroutine and answers with the same id.
6. On failure or timeout the pending override is dropped.

The HTTP request is synchronous end to end. There is no job queue.

Rolling restart is not a server feature. `ApplicationDetail.tsx` loops over the application's pools in the browser, firing one command, waiting 10 s, firing the next. Closing the tab stops the roll.

## Flow 5: metrics

- Prometheus discovers hosts from `GET /prometheus/targets`: every non-terminal machine's first IPv4 on port 9182, labelled `machine_id`, `hostname`, `status`.
- The browser queries Prometheus directly for every chart (`web/src/services/prometheus.service.ts`). The control plane is not a proxy.
- `internal/application/metrics/checker.go` runs every 60 s: three PromQL queries (CPU, memory, lowest free disk), compares with thresholds (default 90/90/10, Helm sets 95/95/5), edge-triggers `high_*`/`low_disk` and their recoveries. Breach state is an in-memory map. Sustained Sev breaches re-fire a reminder at most every 15 min.

## Flow 6: logs

- Logs are opt-in per app pool membership: an operator sets a log directory on an app pool inside an application.
- `BuildFluentConfig` collects every (application, pool, path) for a machine and pushes the full set as `fluent_config`. The agent writes one `conf.d/wimp_pool_<id>.conf` per entry (tail input, Loki output with labels `job=wimp, application_id, pool_id, machine_id, filename`), deletes stale ones, ensures `@INCLUDE conf.d\*.conf` in the main config, and restarts Fluent Bit. Pushed on every register and on every log path change.
- The UI reads logs through the control plane (`/projects/:key/applications/:appId/logs`), which queries Loki.
- Download: the agent zips the log directory (limits 500 MB raw, 100 MB zipped), base64s it over the WebSocket, the control plane stages it on disk with a 10-minute token, and the browser fetches `/downloads/:token`.
- Clear: Loki delete request by label selector (needs compactor deletion enabled), then the agent deletes the files.

## Flow 7: application health checks

- Applications with a `health_check_url` appear in `GET /prometheus/monitors` with per-target `__scrape_interval__` and `__scrape_timeout__` labels. Timeout defaults to 30 s, is clamped to interval minus 5 s, floored at 5 s.
- Blackbox exporter probes them. `internal/application/monitor` (the name is historical) checks `probe_success` and `probe_duration_seconds` every 30 s.
- Two consecutive failures fire `health_check_down`. A succeeding probe slower than 5 s, twice in a row, fires `health_check_slow`. Down clears slow state so a recovery does not emit a stray "slow recovered".
- Counter state is persisted on the `applications` row (`consecutive_failures`, `alert_fired`, `consecutive_slow`, `slow_alert_fired`), unlike the other checkers' in-memory state.
- Health-check alerts carry machine id 0 and subject `application:<id>`.

## Flow 8: SQL Server

Three tiers, each needing more access:

| Tier | Needs | Gives |
|---|---|---|
| State | Nothing | Instances from registry, service state, FCI owner, cluster nodes, AGs (from cluster resources, primary = owner node). Alerts: instance/agent stopped/started, FCI and AG failover, cluster node down/up. |
| Counters | Nothing (bootstrap enables `mssql` collector) | Batch requests, PLE, buffer cache, locks, deadlocks, memory grants, AG queues via windows_exporter |
| DMVs | One SQL login (same on every instance) or Windows auth as `NT AUTHORITY\SYSTEM`, granted `VIEW SERVER STATE`, `VIEW ANY DEFINITION`, `db_datareader` on msdb | Waits, blocking, longest request, DB state, backup age, failed jobs, AG replica health. Alerts: DB unhealthy, backup/log backup overdue, job failed, AG unhealthy, blocking. |

Mechanics:

- Credentials are pushed in a `sql_monitor_config` frame on every register to every agent, held in memory only, never written to disk. Use HTTPS when using a SQL login.
- Exactly one node monitors each instance: a standalone instance where it runs, an FCI on its current owner (connects via the virtual server name). This node sets `monitored=true` and reports `monitor_error`.
- The agent polls DMVs every 60 s and writes `wimp_mssql.prom` atomically into the textfile directory. windows_exporter serves it on the existing scrape. No new port, no new scrape job.
- `internal/application/sqlmetrics` queries those series every 60 s and edge-triggers alerts. AG databases are folded across replicas ("backed up" if any replica has a recent backup).
- Live lists (blocking chains, running requests, jobs) are not time series, so they are fetched on demand through `sql_query` frames when someone opens the tab.
- Actions: clustered instances only through the cluster (`Start-ClusterGroup`, `Move-ClusterGroup`), never by stopping the service, which the cluster would treat as a failure. Standalone restart also starts SQL Agent if its start type is Automatic. AG failover runs on the target replica and refuses unless synchronous-commit and all databases SYNCHRONIZED.
- State alerting (`sqlinstance.Evaluate`) is keyed by scope, not reporting node. A clustered instance's scope is `fci:<virtual server>\<instance>` with machine id 0, so its incident can open on one node and close on another after failover. The first observation of a key sets a baseline without alerting, so a control plane restart does not re-alert everything already down. SQL Agent state is ignored while the engine is down, so one outage is one alert. Operator-initiated start/restart suppresses edges for 5 minutes, then compares: success is silent, failure alerts.

## Flow 9: the alert pipeline

Everything that can alert calls `notification.EmitAlert(type, projectID, machineID, instance, subject, title, detail)` in `internal/application/notification/service.go`:

```
EmitAlert
 ├─ look up AlertTypeRegistry[type]           (category, default severity)
 ├─ severity = settings override  > env/Helm > registry default
 ├─ if severity != disabled:
 │    ├─ insert notifications row
 │    ├─ broadcast to every browser WebSocket ({type:"notification", payload})
 │    └─ if severity >= receiver minimum and alerting enabled and ALERTMANAGERURL set:
 │         POST Alertmanager /api/v2/alerts  (async; labels alertname=title, severity,
 │         instance, category, source=wimp; annotation summary; generatorURL deep link)
 ├─ unless repeat: incident.Record(...)        (open / close / ignore by IncidentBindings)
 └─ if severity == sev and not repeat: recovery.Trigger(...)   (no-op today)
```

Severity order: info < warning < critical < sev. `disabled` is not ranked; it means "do not even create". The alertname label is the full title (`WEB01 // App Pool Stopped: Api`), and Alertmanager is configured with `group_by: ["..."]` (every label), so each machine's alert is its own group and does not wait behind another's group window.

Callers emit from goroutines (`go notificationService.EmitAlert(...)`) so a slow DB or Alertmanager never blocks the heartbeat loop.

## Flow 10: incidents

`internal/domain/notification/alert_types.go` holds two tables side by side: `AlertTypeRegistry` and `IncidentBindings`. A binding maps an alert type to an `IncidentKind` and a role (`opens` or `closes`). Types without a binding (agent updated, planned restart, reassigned, failovers) are informational.

`incident.Record` under one global mutex:

- Opens: skip if suppressed; skip if an open incident already exists for (project, machine, kind, subject); else insert.
- Closes: find the open incident for the same key and resolve it, even if the closing alert type is suppressed (silencing "recovered" must not strand incidents open). Nothing open is normal after a restart.

`subject` is what keeps two failing pools on one host as two incidents. Title and detail of both ends are denormalised onto the incident row so pruning notifications never punches holes in incident history. The timeline shows open incidents plus anything resolved in the last 7 days. Operators can resolve manually (for conditions that will never report their own recovery, such as a pool removed mid-outage).

## Flow 11: agent self-update

Triggered by `POST …/agent/update` or automatically on register. The agent downloads `agent.exe` for the configured version to `agent.update.exe` (rejects < 1 KB), writes `wimp_update.bat` (ping-based 5 s delay, `net stop`, `move /Y`, `net start`, delete task, delete itself), registers it as a SYSTEM scheduled task and fires it with `schtasks /Run`. The delay lets the command result reach the control plane before the service stops. The comments in `internal/agent/update.go` record the two failure modes this design replaced (wall-clock `/ST` scheduling that never fired, and nested quoting in `/TR` that produced a task that ran but did nothing).

## Flow 12: machine identity, reassignment and deletion

- The agent sends Windows `MachineGuid` as `machine_uid` on register.
- `ReconcileReassignment` finds other rows with the same UID (definite match) or, for rows predating UID reporting, the same hostname but only if not currently connected. Matches become `reassigned` with `superseded_by_id`, are excluded from Prometheus SD, and emit `machine_reassigned` in the project the host left.
- Delete: `DELETE …/machines/:id` sets status `deleting` and returns the uninstall command. If offline, the row is hard-deleted immediately; if online, it is hard-deleted when the agent disconnects (after the operator runs the uninstall). See operations.md for a bug in this path.

## Flow 13: authentication and authorisation

- Login: bcrypt password check, then either tokens or, if MFA is enabled on the account, an MFA challenge. TOTP via `/auth/mfa`. Rate limit 10 attempts per 5 min per IP (sliding window in the cache).
- Access token: HS256 JWT, 1 h, claims `sub`, `username`, `roles`, `iss=https://wimp.suei.io/`. Refresh token: 32 random bytes, stored as SHA-256 hash in the cache for 7 days, rotated on every refresh. The SPA stores both in localStorage and refreshes on 401 with a queue so concurrent 401s refresh once.
- Roles: each account carries `internal_roles: {"default": "org_<role>__default"}` built from its account role (SUPERADMIN, ADMIN, GUEST). Handlers call `EnforceRoles(roles, domain, object, action)`, which strips `__entity` and asks Casbin. `data/policy.csv` grants org-level rights per role and maps `org_superadmin` to `project_owner` through `g2`.
- Browser WebSocket `/ws?token=` validates the JWT once at upgrade.
- First-run: `POST /account` and `POST /organization` are public only until one exists.
- MFA enforcement (`ENFORCEMFA` / `security.enforce_mfa`) is resolved and exposed on `GET /config`, but neither the login handler nor the login page acts on it yet. The Settings toggle is intentionally disabled with the note "becomes editable once account enrolment exists".

## Where state lives

This table is the reason the control plane runs as one replica.

| State | Location | Survives restart | Breaks with 2 replicas |
|---|---|---|---|
| Projects, accounts, machines, pools, sites, SQL rows, applications, notifications, incidents, settings | DB | Yes | No |
| Health-check counters | DB (`applications` row) | Yes | No |
| Agent connections | `hub.Get()` map | No (agents reconnect) | Yes: a command lands on the replica without the socket |
| Pending command channels | `hub` command map | No | Yes |
| Browser WebSockets | `hub.Clients()` | No | Yes: notifications reach only some browsers |
| Pending pool/site/machine action overrides | `internal/cache` maps, 15 to 30 s TTL | No | Yes |
| Sidecar health previous state | `internal/cache` | No, first heartbeat sets baseline | Yes |
| Metric breach state, SQL DMV breach state, SQL instance edge/owner/planned maps | Package-level maps | No, re-learned silently | Yes: double alerts |
| Sev repeat timers | `internal/cache` | No | Yes |
| Incident open/close serialisation | Global mutex | n/a | Yes: duplicate incidents |
| Setting overrides | DB, mirrored to an in-memory snapshot | Yes | Yes: a write on one replica is invisible to the other until restart |
| Refresh tokens, MFA challenges, rate limit counters | Cache (memory in Helm) | No with memory: everyone logs out on deploy | Yes with memory; fine with Redis |
| Staged log downloads | Local disk + map, 10 min | No | Yes |

## Wire protocol

Defined once in `internal/domain/protocol/message.go`, imported by both binaries. Envelope `{type, payload}` as JSON text frames.

| Type | Direction | Purpose |
|---|---|---|
| `register` | agent to CP | machine id, hostname, IPs, agent version, Windows version, machine UID |
| `register_ack` | CP to agent | triggers discovery |
| `discovery` | agent to CP | full pools, sites with bindings, SQL instances, cluster |
| `heartbeat` | agent to CP | running pool/site names, SQL + cluster snapshot, sidecar health |
| `command` / `command_result` | both | `{command_id, action, target_type, target, args}`. Target types: `app_pool`, `site`, `machine`, `agent`, `sql_instance`, `availability_group` |
| `fluent_config` | CP to agent | full desired set of log tail configs + Loki connection |
| `list_files` / `_result` | both | log file listing for the viewer |
| `download_logs` / `_result` | both | base64 zip |
| `clear_logs` / `_result` | both | delete files in a log directory |
| `sql_monitor_config` | CP to agent | SQL login or empty for Windows auth |
| `sql_query` / `_result` | both | live `blocking`, `requests`, `jobs` lists |

Compatibility rule followed so far: new fields are additive with `omitempty`, and the control plane tolerates their absence (for example `machine_uid` is empty from older agents and the code falls back to hostname matching). There is no protocol version field.

## Data model

```
Organization 1──* Project 1──* Machine 1──* AppPool
   (one: "default")    │          │  └──* Site
                       │          └──* SqlInstance
                       ├──* Application *──* AppPool        (ApplicationAppPool: log_path)
                       │                *──* SqlInstance    (ApplicationSqlInstance)
                       ├──* Notification
                       ├──* Incident
                       └──* Monitor (dead)
Account (internal_roles JSON, MFA secret)       Setting (key/value overrides)
Metadata (bootstrap token, language)
```

All models embed `gorm.Model` (id, created/updated/deleted timestamps, soft delete). JSON columns via GORM's serializer: machine IPs, machine cluster snapshot, site bindings, account roles, project `created_by`. Uniqueness: (machine, name) for pools, sites and SQL instances; (application, pool) and (application, instance) for memberships. Machine deletion is a hard delete (`Unscoped`).

## HTTP API surface

Grouped; full list in `internal/infrastructure/server/router.go`.

| Group | Auth | Notes |
|---|---|---|
| `/ws/agent`, `/bootstrap`, `/bootstrap/uninstall` | Machine token or none | |
| `/health`, `/config`, `/prometheus/*`, language endpoints, `GET /organization` | None | |
| `/auth/login`, `/auth/mfa` | None, rate limited | |
| `/auth/refresh-token`, `/auth/revoke-token`, `/mfa/totp-uri`, `/mfa/confirm` | None (MFA routes take username+password) | |
| `/account`, `/accounts`, `/organization` writes, `/settings` | JWT, org-domain Casbin check | |
| `/project(s)`, `/projects/:key/...` | JWT, project-domain Casbin check | Machines, pools, sites, SQL, applications, logs, dashboard, incidents |
| `/notifications/*` | JWT | Read/unread state is global, not per user or project |
| `/ws` | JWT in query | Browser live feed |
| `/metrics` | None | Control plane's own Prometheus metrics (`handlers.MetricsHandler`) |
