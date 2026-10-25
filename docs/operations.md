# Operations, security and known issues

## Deploying

The supported install is the Helm chart:

```sh
helm upgrade --install wimp oci://ghcr.io/su3i/charts/wimp -n wimp --create-namespace -f your-values.yaml
```

Name the release `wimp`. The bundled Prometheus scrape configs point at `http://wimp-control-plane/...` and the blackbox replacement address at `wimp-prometheus-blackbox-exporter:9115`; another release name needs those overridden.

What the chart deploys with defaults: control plane (1 replica, LoadBalancer), web (nginx, LoadBalancer), Loki monolithic with MinIO, kube-prometheus-stack (Prometheus + Alertmanager + operator; Grafana, node exporter and kube-state-metrics disabled), blackbox exporter. Postgres is not bundled; the default `database.type` is `postgres` and you supply it. Secrets (JWT secret, bootstrap token, admin credentials, DB password, SQL monitor password) go into one chart-managed Secret.

Minimum values to set: `appUrl`, `webUrl`, `config.jwtSecret`, `config.bootstrapToken`, `config.defaultAdminPassword`, `config.database.*`, `config.lokiHost` (`<release>-loki`), `config.prometheusUrl`, `config.alertmanagerUrl`, `web.apiUrl`, `web.prometheusUrl` (must be browser-reachable), and an Alertmanager receiver in place of `"null"` for `source="wimp"`.

The README mentions a docker-compose install and `.env.example` has a "Docker Compose only" section, but there is no compose file in the repository.

The production deployment's values are kept outside this repo (local permission history references an infrastructure repo that vendors the chart as a dependency).

## Configuration reference

Environment variables, read by `envconfig` with no prefix. Names are the struct field names uppercased.

| Group | Variables | Notes |
|---|---|---|
| App | `APPENV`, `APPURL`, `APPPORT`, `WEBURL` | `APPURL` is what agents dial and what bootstrap commands embed. `WEBURL` sets CORS and alert deep links. `APPENV=development` adds the ngrok header to bootstrap commands. |
| Agent | `AGENTVERSION`, `AUTOUPDATEAGENT` | Version must exist as a GitHub release |
| Auth | `JWTSECRET`, `BOOTSTRAPTOKEN`, `DEFAULTADMINUSERNAME`, `DEFAULTADMINPASSWORD`, `ENFORCEMFA` | Admin is seeded once; changing the env later does not change the account |
| Database | `DATABASETYPE` (`sqlite` default), `DATABASEPATH`, `DATABASEHOST`, `DATABASEPORT`, `DATABASENAME`, `DATABASEUSERNAME`, `DATABASEPASSWORD`, `DATABASEUSESSL` | |
| Cache | `CACHETYPE` (`memory` default), `REDISADDR`, `REDISPASSWORD`, `REDISDB` | The chart does not set these |
| Loki | `LOKIHOST`, `LOKIPORT` (required), `LOKITLSENABLED`, `LOKITLSSKIPVERIFY` | Also sent to agents for Fluent Bit |
| Integrations | `PROMETHEUSURL`, `ALERTMANAGERURL` | Empty Prometheus URL disables all three checkers |
| SQL | `SQLMONITORUSERNAME`, `SQLMONITORPASSWORD` | Empty = Windows auth as SYSTEM |
| Casbin | `MODELCONFPATH`, `POLICYCSVPATH` | Read into config but ignored: the enforcer hardcodes `./data/model.conf` and `./data/policy.csv` |
| Alerts | `ALERTSEVERITY_<TYPE>` for each type, `ALERTRECEIVERMINSEVERITY` | Overridable at runtime from Settings |
| Thresholds | `THRESHOLD_HIGHCPU_PERCENT`, `THRESHOLD_HIGHMEMORY_PERCENT`, `THRESHOLD_LOWDISK_PERCENT`, `THRESHOLD_SQL_BACKUP_HOURS`, `THRESHOLD_SQL_LOG_BACKUP_MINUTES`, `THRESHOLD_SQL_BLOCKING_SECONDS`, `THRESHOLD_HEALTHCHECK_SLOW_SECONDS`, `HEALTHCHECK_TIMEOUT_SECONDS` | Not overridable at runtime |

### Severity defaults disagree across files

The registry in code, `.env.example` and `helm/values.yaml` give different defaults. For a Helm install the Helm column is what runs (unless overridden in Settings).

| Alert | Registry | .env.example | Helm values |
|---|---|---|---|
| machine_disconnected | sev | sev | critical |
| app_pool_stopped | critical | critical | warning |
| site_stopped | critical | critical | warning |
| windows_exporter_down, fluent_bit_down | critical | critical | warning |
| health_check_down | critical | critical | sev |
| high_cpu, high_memory | warning | warning | critical |
| Thresholds CPU / memory / disk | 90 / 90 / 10 | 90 / 90 / 10 | 95 / 95 / 5 |
| Receiver minimum | warning (fallback) | warning | critical |

Practical effect on Helm installs: the only things that page out of the box are Critical and Sev, a host going offline is Critical (so it does not trigger the recovery hook), and an application health check failing is Sev.

## Onboarding and offboarding hosts

1. Hosts > Add host. Copy the two commands.
2. On the host, in an elevated PowerShell, in the directory where WIMP should live (bootstrap installs relative to the current directory: `.\wimp`, `.\fluent-bit`, `.\tmp`, `.\textfile`), run both.
3. The script reports `[OK]`/`[FAIL]` per service. The host appears online within seconds.

Requirements on the host: outbound HTTPS to the control plane, GitHub, the Fluent Bit package site and Loki; inbound 9182 from Prometheus.

Re-run bootstrap after installing IIS, SQL Server or clustering later, to add collectors. Re-running bootstrap from another project's command moves the host: the old row becomes `reassigned`.

Removing a host: Hosts > Delete gives an uninstall command (`/bootstrap/uninstall` serves `data/cleanup.ps1`). See the known issues below for two problems in this path.

## Upgrading

- Control plane and web: `scripts/bump-app.sh`, push, wait for images, `helm upgrade`. Schema changes apply on boot via AutoMigrate. Every agent reconnects; users are logged out (memory cache).
- Agents: `scripts/bump-agent.sh`, push, wait for the GitHub release to exist, then roll out the new `agentVersion` in values. With `autoUpdateAgent: true` every host updates on next connect, which after a control plane restart means all at once. Without it, update hosts one by one from the host page.

## SQL Server access

Instance state, failover and cluster alerts need nothing. For DMV metrics, on every instance:

```sql
-- [login] is the SQL login, or [NT AUTHORITY\SYSTEM] for Windows authentication
GRANT VIEW SERVER STATE TO [login];
GRANT VIEW ANY DEFINITION TO [login];
USE msdb;
CREATE USER [login] FOR LOGIN [login];
ALTER ROLE db_datareader ADD MEMBER [login];
```

Planned AG failover additionally needs `ALTER ANY AVAILABILITY GROUP` (SYSTEM normally has it on AG replicas). Start, restart and move use Windows and cluster permissions only. A login or permission error surfaces in the UI as "configure SQL credentials" via the instance's `monitor_error`.

## Security posture

What is in place:

- Agents dial out; hosts expose no control endpoint.
- Every project route requires a JWT and a Casbin check in the handler. Ownership of machines/applications is checked against the project key.
- bcrypt passwords with a complexity rule (8+ chars, upper, lower, digit, symbol). TOTP MFA per account.
- Login and MFA verification rate limited (10 per 5 min per IP).
- CORS restricted to `WEBURL` and the Vite dev origin.
- First-run account and organization creation closes once one exists.
- Refresh tokens hashed at rest and single-use.
- SQL credentials never written to host disk.
- No remote shutdown, no remote SQL stop.

Open items, roughly by severity. Several are ROADMAP items; the rest were found while writing these docs and are not tracked anywhere else yet.

| # | Issue | Where | Impact |
|---|---|---|---|
| S1 | Agent tokens never expire and cannot be rotated | `application/machine/service.go` | Leaked token = permanent impersonation of that host until the row is deleted. Token travels in URL query strings. |
| S2 | Authorisation is not project-scoped | `application/authorization/service.go`, `data/policy.csv` | Role suffix (the entity key) is discarded; any account allowed on "project" is allowed on every project. Fine while one org = one team. |
| S3 | Only SUPERADMIN can use any project endpoint | `data/policy.csv` | `org_admin` and `org_guest` have org rights only; `project_owner/editor/viewer` are defined but never assigned. ADMIN and GUEST accounts can log in and see nothing project-scoped. |
| S4 | `EnforceRoles` returns after evaluating the first role | `application/authorization/service.go` | Harmless today (each account has one role), wrong as soon as an account has two. |
| S5 | Prometheus is exposed to browsers without auth | `web.prometheusUrl` | Anyone who can reach it can read every metric and run arbitrary PromQL. |
| S6 | Unauthenticated discovery endpoints | `/prometheus/targets`, `/prometheus/monitors` | Leak hostnames, IPs and health check URLs. Should be cluster-internal only. |
| S7 | Agent update binary not verified | `internal/agent/update.go` | Trusts HTTPS to GitHub and size > 1 KB. No checksum or signature. Runs as SYSTEM. |
| S8 | `/mfa/totp-uri` and `/mfa/confirm` are not rate limited | `router.go` | Both accept username+password, so they are an unthrottled password-guessing path around the `/auth/login` limit. |
| S9 | MFA enforcement is not enforced | `setting.EnforceMfa()` is only exposed on `/config` | Turning it on changes nothing yet. |
| S10 | WebSocket upgrader accepts any origin | `handlers/ws.go` | Browser socket still needs a valid JWT, so impact is limited. |
| S11 | No audit trail | | Cannot answer "who restarted that pool". ROADMAP item 6. |
| S12 | SQL login sent over the agent socket | | Plain `ws://` exposes it. Use `https` for `APPURL`. |

## Known bugs and rough edges

Found by reading the code; not reproduced.

| # | Issue | Detail |
|---|---|---|
| B1 | Deleting an online host can be undone by its own heartbeat | `RequestDeletion` sets status `deleting`, but the connection's heartbeat handler does a full `Save` of its in-memory machine copy (status `online`) every 30 s. If the uninstall runs more than ~30 s after the delete click, the deferred disconnect handler re-reads `online`, marks the row offline instead of hard-deleting it, and emits a `machine_disconnected` alert. The register path already avoids this with a scoped `SetIdentity` update; the heartbeat path needs the same (update `last_seen_at` and `cluster` only). |
| B2 | Uninstall leaves the agent directory | `cleanup.ps1` removes `.\agent`, bootstrap installs to `.\wimp`. |
| B3 | Users are logged out on every control plane restart | Helm never sets `CACHETYPE`, so refresh tokens live in process memory. Rate limit counters reset too. Setting `CACHETYPE=redis` fixes it but needs a Redis the chart does not provide. |
| B4 | Missed recoveries leave incidents open | If a condition clears while the control plane is down, the in-memory baseline after restart sees "up" and emits nothing. Manual resolve covers it. |
| B5 | Notification read state is global | `UnreadCount` and `MarkAllRead` ignore project and user. One person reading clears it for everyone. |
| B6 | Dead code | `domain/monitor`, `handlers/monitor.go`, `web/src/pages/Monitors.tsx` (no routes), `application/telegram` (unused). README still advertises standalone uptime monitors and docker-compose. |
| B7 | Rolling restart has no health gate | Fixed 10 s gap, browser-driven, stops if the tab closes. |
| B8 | Alertmanager auto-resolves after 5 min | See decisions.md entry 5. |
| B9 | Docs drift | `ROADMAP.md` still says zero tests, open CORS, no rate limiting, public account endpoints. All fixed. |
| B10 | Hardcoded release name in Prometheus SD URLs | `values.yaml` |
| B11 | Fluent Bit restarts on every agent reconnect | Config push on register always restarts it, even when nothing changed. |

## Runbook notes

- **Host shows offline but the box is fine:** check the `wimp-agent` service on the host, then outbound reach to `APPURL`. The agent retries every 30 s at most. Logs go to the Windows service logger (Event Log).
- **Host online but no metrics:** windows_exporter not running (there is an alert for this), port 9182 blocked, or the host's first IPv4 is not the one Prometheus can reach.
- **SQL instance says "configure SQL credentials":** the monitoring node's last DMV poll failed with a login or permission error. Check grants on that instance.
- **Logs not appearing:** the app pool must belong to an application and have a log path set; check `conf.d\wimp_pool_<id>.conf` exists under the Fluent Bit directory on the host and that the host reaches Loki.
- **"Clear logs" returns 502 mentioning compactor:** Loki deletion is not enabled on an externally managed Loki.
- **Agent update did nothing:** look for the `WimpAgentUpdate` scheduled task and `wimp_update.bat` next to `agent.exe`; confirm the GitHub release for that version exists.
- **An incident is stuck open:** resolve it manually from Incidents. It does not touch the underlying condition.
