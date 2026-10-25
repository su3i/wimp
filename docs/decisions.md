# Decisions and trade-offs

Each entry: what was chosen, what it was chosen over, what it buys, what it costs, and when to revisit. Where the reasoning is written in the code, the file is cited. Where it is inferred from the design, it says so.

The thread running through all of them: **one engineer runs this, in production, for a real fleet.** Every choice favours fewer moving parts to operate, behaviour that is obvious at 2 am, and reusing mature open-source pieces over building them. Scale-out and abstraction are deferred until something actually needs them, and the places where that bet has a ceiling are marked with `ponytail:` comments.

---

## 1. Agent dials out over a WebSocket

**Chosen over:** WinRM/PowerShell Remoting, SSH on Windows, an HTTP API listening on each host, RDP automation.

**Why:** Nothing on the host has to accept control traffic. No inbound firewall rules, no WinRM listener exposed on the network, no credential for the control plane to hold per host. Hosts behind NAT or in other networks work as long as they can reach one URL. A persistent socket gives presence for free (connection open = online) and lets the server push commands with no polling delay.

**Gain:** Simple onboarding (one outbound URL), smaller attack surface on hosts, instant online/offline detection (90 s worst case via read deadline), sub-second command delivery.

**Cost:** The control plane holds state per connection, which is the main reason it runs as one replica (see 9). Commands are synchronous RPC over a socket with a 30 s ceiling, no queue, no retry, no delivery to offline hosts. Metrics scraping is still inbound to the host on 9182, so "no inbound ports" is true for control, not for metrics.

**Revisit when:** a second replica is needed (route commands via a shared bus, or shard agents by machine id), or a host cannot expose 9182 (switch windows_exporter to push via the agent, or remote-write).

## 2. Go for both agent and control plane, one module

**Chosen over:** .NET for the agent (natural on Windows), Node/Python for the server.

**Why (inferred):** A single static `agent.exe` with no runtime to install, cross-compiled from a Mac/Linux CI runner with `GOOS=windows CGO_ENABLED=0`. One language for both sides means the wire protocol is one Go package (`internal/domain/protocol`) imported by both binaries, so they cannot drift.

**Gain:** Trivial agent distribution (download one file), low memory footprint on hosts, shared types, one toolchain.

**Cost:** Windows-specific work (IIS, cluster, SQL service control) is done by shelling out to `appcmd.exe`, `sc.exe` and PowerShell rather than native APIs, and output is parsed. That is slower (PowerShell per heartbeat on cluster nodes is ~1 s CPU per 30 s, marked `ponytail:`) and brittle across Windows versions.

## 3. Compose existing open-source tools for metrics, logs and delivery

**Chosen over:** Building metric storage, log storage and notification integrations into WIMP.

**Why:** windows_exporter, Fluent Bit, Prometheus, Loki, blackbox exporter and Alertmanager are mature, well understood, and free. WIMP's value is the control plane and the domain model on top, not a time series database.

**Gain:** Months of work avoided. Users who already run Prometheus/Loki/Alertmanager can point WIMP at theirs (every subchart is optional). Grafana and PromQL work on the same data.

**Cost:** A WIMP install is really seven services, and Loki needs object storage (MinIO bundled; MinIO images moved to a community rebuild mid-2026 and the chart had to follow). Alerting on metrics is done by polling Prometheus from Go rather than with Prometheus rules, so thresholds live in WIMP config rather than in PromQL rule files.

## 4. Prometheus discovers hosts from the control plane (HTTP SD)

**Chosen over:** Static scrape configs, file SD, Kubernetes SD, Consul.

**Why:** The control plane already knows every host, its IP and its status. Serving that as HTTP SD means adding a host in the UI is enough; Prometheus picks it up within 60 s. Terminal rows (reassigned, deleting) are filtered out so a host that moved projects stops charting under the old id (`handlers/prometheus.go`).

**Gain:** Zero Prometheus configuration per host. Labels (`machine_id`, `hostname`) are WIMP's own ids, so queries join cleanly to the DB.

**Cost:** The SD endpoints are unauthenticated and list hostnames, IPs and health check URLs. The scrape config in `values.yaml` hardcodes `http://wimp-control-plane/...`, which assumes the release is named `wimp`. Uses the first IPv4 only.

## 5. Alertmanager is the only outbound channel

**Chosen over:** Built-in Telegram/Slack/email/PagerDuty integrations. There is a `telegram` package in the code from before this decision; it is unused.

**Why:** Alertmanager already does routing, grouping, silencing, inhibition and every receiver type. WIMP emits alerts with consistent labels (`source=wimp`, `severity`, `category`, `instance`) and lets the operator route them.

**Gain:** Any receiver Alertmanager supports, with no WIMP code. Silencing and maintenance windows come free.

**Cost:** Message formatting lives in Alertmanager templates (the Telegram template is shipped as a commented example in `values.yaml`). Two things WIMP had to work around: Alertmanager groups by labels, so WIMP sets `alertname` to the full title and the chart uses `group_by: ["..."]` so ten machines alerting at once are ten notifications, not one plus nine waiting on the group window. Alerts are posted once, as firing, with no `endsAt`. Alertmanager therefore auto-resolves every WIMP alert after `resolve_timeout` (5 min in the chart), regardless of the real condition. If a receiver has `send_resolved` on, its "resolved" message means "5 minutes passed", not "fixed". Recovery is communicated by WIMP's own separate recovery alert, which is Info and usually below the receiver minimum.

## 6. One alert chokepoint, one registry, incident bindings next to it

**Chosen over:** Alert logic scattered per feature; incidents as a separate rule system.

**Why:** Written in `application/notification/service.go` and `domain/notification/alert_types.go`. Every alert-worthy event calls `EmitAlert`. Severity resolution, persistence, browser broadcast, Alertmanager delivery, incident tracking and the recovery hook happen in one place, so a new alert type gets all of it by default. `IncidentBindings` lives in the same file as the registry "deliberately: adding an alert type without deciding whether it opens, closes, or does neither is exactly the omission that leaves incidents dangling, and keeping both tables in one file makes that omission visible in review."

**Gain:** Uniform behaviour, one place to add audit logging or recovery later, the full alert catalogue readable in one screen.

**Cost:** Titles and details are free text built at each call site, so formatting consistency depends on discipline (`AlertTitle` helps).

## 7. Five severities, with Sev and Disabled

**Chosen over:** The usual three (info/warning/critical).

**Why:** Critical in practice gets used for "should look at today". Sev exists so "drop everything" can be routed and treated differently, and it is the trigger for the future recovery engine. Disabled exists so an operator can opt out of an event entirely: not created, not shown, opens no incident. A suppressed recovery still closes its incident, an asymmetry documented in `application/incident/service.go`, because silencing "high CPU recovered" should not strand every CPU incident open.

**Gain:** A clean escalation path and a natural hook for automation. Noise can be cut per alert type without losing incident accuracy.

**Cost:** Defaults now live in three places that do not agree: the registry, `.env.example`, and `helm/values.yaml` (see operations.md). The Helm values are the effective defaults for most installs.

## 8. Edge-triggered alerting with in-memory state, baseline on first sight

**Chosen over:** Level-triggered alerts on every check; persisting breach state in the DB.

**Why:** Written in `application/metrics/checker.go` and `application/sqlinstance/service.go`. An alert fires on the transition, the recovery fires on the reverse transition, and nothing fires while the state holds (except a 15-minute reminder for sustained Sev breaches). The first observation of a key after a restart only sets a baseline, "so a control plane restart re-learns state silently instead of re-alerting everything that is already down."

**Gain:** No alert storms, simple code, restarts are quiet.

**Cost:** A condition that recovers while the control plane is down produces no recovery alert, and its incident stays open until manually resolved or until it fails and recovers again. Health checks are the exception: their counters live on the `applications` row.

## 9. Exactly one control plane replica

**Chosen over:** Horizontal scaling with shared state.

**Why:** Stated in several comments (`application/incident/service.go`, `application/sqlinstance/service.go`, the Helm `replicaCount: 1`): agent sockets, pending commands, browser sockets, breach maps, pending action overrides and the incident mutex are all in process. The chart is sized for "a fleet in the low hundreds of hosts" (256 Mi request, 512 Mi limit) because memory scales with connections, not requests.

**Gain:** No distributed coordination, no message bus, no Redis requirement. Concurrency bugs are local and testable.

**Cost:** Restarts and deploys drop every agent connection (they reconnect within seconds with backoff) and, with the default memory cache, log every user out. No HA. The full list of what breaks with two replicas is in architecture.md.

**Revisit when:** the fleet passes a few hundred hosts or HA becomes a requirement. The path: Redis for the cache (already supported), move incident open/close to a conditional insert, move breach state into the DB or Redis, and route commands to the replica holding the socket.

## 10. Settings: deployment config is the default, DB overrides win

**Chosen over:** Env only (needs a redeploy to change a severity) or DB only (loses GitOps).

**Why:** Written in `domain/setting/model.go` and `application/setting/service.go`. Helm values stay the source of truth for anything nobody touched. The `settings` table stores only what an operator changed in the UI, as key/value rows "because settings are added constantly, a typed row would need a migration every time". Reads go to an in-memory snapshot because severity lookup is on the path of every alert.

**Gain:** Operators can tune alert noise live, the chart stays declarative, and the UI can show "overridden" vs "from deployment".

**Cost:** Two sources to check when someone asks "why is this alert Critical". The snapshot is per process (fine with one replica).

## 11. Postgres or SQLite through GORM, AutoMigrate, duplicated repositories

**Chosen over:** One database only; a migration tool (goose, atlas); a query builder like sqlc.

**Why (inferred):** SQLite makes a single-binary local run and small installs trivial; Postgres is the production default in Helm. GORM gives both from one model definition. AutoMigrate removes a migration step from releases.

**Gain:** Zero-ops schema changes for additive work, one-command local setup.

**Cost:** Two nearly identical repository trees to keep in sync. AutoMigrate never drops or renames. JSON columns are typed `jsonb` in tags even for SQLite. No migration history.

**Revisit when:** a column needs renaming or a data backfill. At that point introduce a migration tool, and consider collapsing the two repository trees into one with dialect switches only where SQL differs.

## 12. JWT + refresh tokens in the cache, Casbin for RBAC

**Chosen over:** Server sessions; an IdP (OIDC) integration; hand-rolled role checks.

**Why (inferred):** Stateless access tokens keep handlers simple. Refresh tokens are hashed and stored in the cache with rotation on every use, so a stolen refresh token is single-use. Casbin with a domain model leaves room for org-level and project-level roles.

**Gain:** Standard, small, works without external dependencies.

**Cost:** As implemented the RBAC is coarser than the model suggests: only SUPERADMIN can reach project endpoints, permissions are not scoped to a specific project, and `EnforceRoles` evaluates only the first role (details in operations.md). No SSO. Tokens in localStorage.

## 13. Agent tokens never expire

**Chosen over:** Short-lived tokens with rotation.

**Why:** Stated in `application/machine/service.go`: "there's no rotation path for it, so it just doesn't expire." Expiry without rotation would brick agents.

**Cost:** A leaked token is permanent until the machine row is deleted. It appears in the bootstrap URL and the agent's WebSocket query string (so in proxy logs) and in `config.json` on the host. Rotation is roadmap item 1.

## 14. Discover, never register; derive roles on read

**Chosen over:** Operators declaring what each host runs.

**Why:** Written in `domain/protocol/message.go` and `application/machine/service.go`: "Every section is empty on a host without that workload, which is how the control plane learns what a host is for without the operator having to say." Host roles (`iis`, `sql`, `cluster`) are computed from rows on every read "so it can never disagree with the rows it is derived from."

**Gain:** Onboarding is one command, the UI is never stale relative to the host, and nav adapts per project.

**Cost:** New workloads after bootstrap need the bootstrap re-run for windows_exporter collectors.

## 15. Some actions are deliberately missing

**Chosen over:** Full parity with what an admin can do over RDP.

**Why:** Stated in `router.go`: "a host shut down remotely stays down until someone physically powers it back on." SQL stop is excluded for the same reason. Clustered SQL is only ever started, restarted or moved through the cluster, because stopping the service directly "would look like a failure to the cluster, which would fail it over or restart it out from under the command." AG failover refuses unless the target is synchronous-commit and every database is SYNCHRONIZED, to prevent data loss.

**Gain:** The browser cannot be used to cause an outage that needs hands to fix. Trust from operators.

**Cost:** Occasional "why can't I…" requests. The answer is a feature, not a gap.

## 16. Planned actions do not page

**Why:** A requested host restart marks the machine for 15 s so the disconnect becomes `machine_restarting` (Info, no incident) instead of `machine_disconnected` (Sev). A requested SQL start/restart suppresses edges for 5 minutes and then compares: success is silent, failure alerts. Agent updates are deliberately not masked, "so it isn't silently masked if the update never comes back online."

**Gain:** On-call trusts that a page means something unplanned.

## 17. Rolling restart runs in the browser

**Chosen over:** A server-side job with progress tracking.

**Why (inferred):** It reuses the existing per-pool command endpoint with a 10 s sleep between pools. No job table, no worker, no progress API.

**Gain:** Shipped in a few lines.

**Cost:** Closing the tab or losing the network stops the roll halfway. No health gating between pools (it waits a fixed 10 s, it does not check the pool came back). No record of who rolled what. This is a natural first workflow for the recovery engine to absorb.

## 18. SQL Server monitoring design

Several linked choices, all documented in `internal/agent/sqlmon.go`, `sql.go` and `application/sqlinstance`:

- **Credential-free tier first.** Instances from the registry, state from services or cluster resources, AGs from cluster resources. Works with zero SQL permissions, works when the instance is stopped (it shows as stopped instead of vanishing).
- **DMV metrics through windows_exporter's textfile collector.** "No new port, no new scrape config." The agent writes a `.prom` file atomically every 60 s.
- **One monitoring node per instance.** Standalone where it runs, FCI on its owner. Prevents N nodes querying the same instance and N copies of each series.
- **Credentials pushed over the socket, held in memory.** Never on the host's disk. Costs: HTTPS is mandatory when using a SQL login, and one login with the same password on every instance.
- **Clustered instances belong to the cluster, not the node.** Incidents open on one node and close on another after failover.
- **Live lists on demand.** Blocking chains, running requests and job lists are point-in-time, so they are queried when someone opens the tab, not stored in Prometheus.

**Gain:** SQL Server coverage without a separate SQL monitoring product, and without adding moving parts to the install.

## 19. Health checks: generous timeout, separate "slow" alert, two strikes

**Why:** Stated in `handlers/prometheus.go` and `application/monitor/service.go`. A blackbox probe that times out is indistinguishable from a refused connection, so a tight timeout reports a slow site as down. The timeout defaults to 30 s (clamped below the interval, since Prometheus rejects a timeout above the interval), and a separate `health_check_slow` alert catches "up but degraded" above 5 s. Two consecutive breaches are needed before either fires.

**Gain:** Down means down. Slow is routed separately.

## 20. Agent self-update through a scheduled task and a batch file

**Chosen over:** The service replacing its own binary; a detached helper process; an MSI.

**Why:** Documented at length in `internal/agent/update.go`, including the two failed designs it replaced. A running service cannot overwrite its own executable. A SYSTEM scheduled task fired with `/Run` stops the service, swaps the file and starts it, decoupled from the old process's lifetime. `ping` is used as the delay because `timeout` fails without a console.

**Gain:** Fleet-wide upgrades by changing one Helm value.

**Cost:** No checksum or signature verification of the downloaded binary beyond "HTTP 200 and > 1 KB". No staged rollout: `AUTOUPDATEAGENT=true` updates every host on its next connect. No automatic rollback.

## 21. Machine identity by Windows MachineGuid

**Why:** Stated in `domain/machine/model.go`. Re-running bootstrap on a host (usually to move it to another project) creates a new row with a new token; the old row would sit offline forever, indistinguishable from a crash, and keep feeding Prometheus a target. The MachineGuid survives that. Not unique in the DB because two rows legitimately share it for the moment between re-bootstrap and reconciliation. Hostname fallback for old agents only matches rows that are not currently connected, so two different boxes with the same hostname in two projects cannot retire each other.

## 22. Logs: per-pool Fluent Bit configs reconciled by the agent

**Chosen over:** One static Fluent Bit config per host; shipping everything under `inetpub\logs`.

**Why (inferred):** Operators choose which directories matter by setting a log path on an app pool inside an application. The control plane pushes the complete desired set; the agent writes one file per entry and deletes the rest, so the host converges on every push. Labels carry WIMP ids, so Loki queries map straight to applications and pools.

**Cost:** Fluent Bit is restarted on every config push (every agent reconnect). "Clear logs" depends on Loki's compactor deletion being enabled (the chart enables it).

## 23. Frontend: SPA on nginx, runtime URL substitution, browser talks to Prometheus

**Chosen over:** Server-side rendering; serving the SPA from the Go binary; proxying Prometheus through the control plane.

**Why:** Written in `web/dockerfile` and `docker-entrypoint.sh`. One image for every environment: the JS is built with placeholders, and the entrypoint substitutes `VITE_API_BASE_URL` and `VITE_PROMETHEUS_URL` at start, then pre-gzips. Charts query Prometheus directly so the control plane does not carry chart load.

**Gain:** Small images, fast loads (vendor chunk splitting, route-level code splitting, `gzip_static`), no chart traffic through Go.

**Cost:** Prometheus must be reachable from every user's browser, unauthenticated. PromQL lives in React components.

## 24. Incidents are derived from alert pairs

**Chosen over:** A separate incident rule engine or manual incident creation.

**Why:** Stated in `domain/incident/model.go`: "every fault-shaped alert in the system already exists as a pair - something breaks, something recovers." An incident is the span between them, so "how long was CPU pinned on WEB01" becomes a stored fact. Both ends are denormalised so pruning notifications never damages incident history.

**Gain:** Incident tracking with no extra configuration, and the dataset the recovery engine and MTTR reporting need.

## 25. Recovery is a seam, not a half-built engine

**Why:** Stated in `application/recovery/recovery.go` and ROADMAP: recovery is "a multi-step workflow (try X, check, try Y)… explicitly out of scope for now." Shipping exactly one call site that every Sev alert reaches means the engine has an obvious place to attach later without reworking the pipeline.

**Cost:** None today. It logs one line.

## 26. Desktop only, dark only

**Why (inferred):** The audience operates from a workstation; dense tables and multi-pane log views do not survive a phone. A blocking overlay is cheaper and more honest than a broken responsive layout. Mobile view is on the owner's feature list, not the roadmap.

## 27. Releases are triggered by version bumps in workflow files

**Why:** The image and agent workflows trigger on changes to their own file, and the bump scripts edit those files. One command updates every place a version is written (Helm, Chart, package.json, health endpoint, env example) and the push ships it. The scripts check every substitution before writing anything.

**Cost:** No CI on pull requests, no test gate, releases cannot be built from a tag. Fine for one engineer pushing to `main`.
