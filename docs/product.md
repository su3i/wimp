# Product

## The problem

Teams that run production on Windows Server and IIS usually operate it by hand. When something breaks, someone opens an RDP session, checks the process, reads a log file, and restarts an app pool. Then they do it again on the next machine. Which machine runs what lives in a spreadsheet or in someone's head. On three servers this is tolerable. On fifty or a few hundred, mean time to recovery is bounded by how fast a person can hop between RDP windows.

The modern observability stack (Prometheus, Loki, Grafana, Alertmanager) solves half of this: it can show metrics and logs. It cannot restart an app pool, it does not know what an IIS site or a SQL failover cluster instance is, and wiring it up for Windows hosts is a project in itself. Commercial APMs see the problem but are priced per host, are SaaS, and still do not give you IIS control. SCOM does both but is heavy and old. WIMP sits in that gap.

## What WIMP is

A self-hosted platform that gives a Windows/IIS/SQL Server fleet one browser tab for three things:

1. **See**: live host status, discovered IIS and SQL Server inventory, CPU/memory/disk/network, IIS and app logs, SQL Server health, application health checks.
2. **Control**: start, stop, restart, recycle app pools and sites; rolling restart of a whole application; restart a host; start/restart/move SQL instances; planned AG failover; agent self-update across the fleet.
3. **Be told**: one alert pipeline with five severity tiers, incidents that open and close themselves, outbound delivery through Alertmanager to wherever the team already lives (the deployment uses Telegram).

The fourth step, **recover**, is the stated direction for v2 and has a placeholder in code (`internal/application/recovery`).

## Who it is for

- Operators and SREs at organisations whose production workloads are .NET on IIS, often with SQL Server behind them, on fleets from a handful to a few hundred Windows servers.
- Teams that cannot or will not send telemetry to a SaaS, which is why everything is self-hosted on Kubernetes via one Helm chart.
- The owner's own production fleet first. The product shape (Telegram as receiver, 20+ IIS hosts, SQL Server with failover clusters and AGs added in October 2026) tracks what that fleet needed. Features land when the fleet needs them, then get generalised.

## Business model and positioning

There is no billing, licensing, telemetry or tenant-signup code in the repo. WIMP is MIT-licensed and open source. Multi-project isolation lets one deployment serve several teams or environments, but there is no SaaS mode. Treat questions about monetisation as open; the repo does not answer them.

How the owner positions it against alternatives (this is framing derived from the README and the design, not text that exists in the repo):

| Alternative | What it lacks that WIMP has |
|---|---|
| RDP + PowerShell + spreadsheet | Everything. This is what WIMP replaces. |
| DIY Prometheus/Loki/Grafana | No control actions, no IIS/SQL domain model, no auto-discovery, no install story for Windows, no incident model. WIMP actually bundles this stack and adds the control plane on top. |
| Datadog, New Relic and similar | SaaS, per-host pricing, no IIS/SQL control actions. |
| SCOM | Heavyweight, Windows-infrastructure-heavy to run itself. WIMP is one Helm install and a one-line PowerShell per host. |
| Ansible, PowerShell DSC | Configuration management, not live operations. No metrics, logs or alerting. |
| WinRM-based tooling | Needs inbound ports on every host. WIMP's agent dials out. |

## Feature inventory (as of 1.1.1)

Status reflects the code, not the README.

| Area | Feature | Status |
|---|---|---|
| Fleet | Add host, get one-line PowerShell bootstrap | Shipped |
| Fleet | Auto-discovery of app pools, sites, bindings, physical paths, SQL instances, cluster, AGs | Shipped |
| Fleet | Online/offline/pending status via 30 s heartbeat; hostname, IPs, Windows version, agent version | Shipped |
| Fleet | Remote host restart (with confirm) | Shipped |
| Fleet | Remote host shutdown | Deliberately not implemented (route returns 501) |
| Fleet | Agent self-update: per host on demand, or fleet-wide on connect via `AUTOUPDATEAGENT` | Shipped |
| Fleet | Re-bootstrap detection: host moved to another project retires its old row | Shipped |
| Fleet | Delete host (returns uninstall command, hard-deletes on disconnect) | Shipped, with a bug (see operations.md) |
| IIS | Start/stop/restart/recycle app pools, start/stop/restart sites | Shipped |
| IIS | Rolling restart/recycle across an application, one pool every 10 s | Shipped (client-side loop in the browser) |
| Applications | Group pools and SQL instances across hosts, aggregated health "5/5 healthy" | Shipped |
| Applications | HTTP health check per application (blackbox exporter), down and slow alerts | Shipped |
| Applications | Log viewer per application (host + pool filter, search, infinite scroll, full screen), download as zip, clear | Shipped |
| SQL Server | Discovery of standalone, FCI and AG without credentials | Shipped |
| SQL Server | Instance/Agent state alerts, FCI and AG failover alerts, cluster node alerts | Shipped |
| SQL Server | DMV metrics (waits, blocking, longest request, DB state, backup age, failed jobs, AG health) with a SQL login or Windows auth | Shipped |
| SQL Server | Start/restart instance, move FCI, planned AG failover (guarded) | Shipped |
| SQL Server | Stop instance | Deliberately not implemented |
| Metrics | Per-host CPU/mem/disk/net charts, fleet CPU comparison, dashboard | Shipped (browser queries Prometheus directly) |
| Alerting | 43 alert types, 5 severities, per-type override via env/Helm and via Settings UI | Shipped |
| Alerting | Sev repeat reminder every 15 min for sustained metric breaches | Shipped (host metrics and SQL DMV checkers; state-driven alerts from heartbeats do not repeat) |
| Incidents | Auto open/close from alert pairs, manual resolve, 7-day timeline | Shipped (1.0.5 to 1.0.8) |
| Settings | Operator overrides for alert severities, receiver minimum severity, alerting on/off, enforce MFA | Shipped (1.0.8) |
| Access | JWT auth, refresh tokens, TOTP MFA, optional enforcement, Casbin RBAC, login rate limiting | Shipped, but RBAC is coarse (see operations.md) |
| Access | Multi-project isolation | Shipped for data; not for permissions |
| Uptime | Standalone uptime monitors decoupled from applications | Dead code. Model, handlers and `Monitors.tsx` exist but no routes are registered. Health checks moved into applications in 0.0.38. The README still advertises it. |
| Recovery | Hook called on every Sev alert | Placeholder (logs one line) |
| Audit | Who did what | Not started |
| Event Log | Windows Event Log | Partially: bootstrap's Fluent Bit config ships Application and System channels to Loki, but nothing in the UI or alerting reads them |
| Mobile | | Blocked on purpose: a full-screen "Desktop only" overlay below the `lg` breakpoint |
| Linux hosts | | Data model accepts `os_type=linux`; no agent exists, API returns 400 |

## Product principles visible in the code

These are the rules the existing features follow. New work should follow them too.

- **Destructive actions that cannot be undone remotely are not offered.** No remote shutdown (the box stays off until someone powers it on). No remote SQL stop. Planned AG failover refuses unless the target is synchronous-commit and every database is SYNCHRONIZED.
- **Alerts must be honest.** A restart the operator asked for does not page anyone unless the thing fails to come back. Repeat reminders are not counted as new incidents. A suppressed alert type opens nothing, but a suppressed recovery still closes its incident. One unlucky health probe does not page (two consecutive breaches required).
- **Discover, don't register.** Operators never type in app pool names, SQL instances or cluster membership. The agent reports what is there.
- **Show only what applies.** Navigation for Applications or SQL Server appears only when some host in the project runs that workload.
- **Problems first.** Lists sort failing things to the top; values are coloured against the same thresholds that alerts use, resolved in one place so UI and alerting cannot disagree.
- **New screens copy the Hosts/Applications pattern** (list page with table, status cells, row menu; detail page with back link, header, actions top-right, tabs, stat cards, 30-minute charts). Two attempts at novel layouts for SQL Server were rejected before the copy-the-pattern version was accepted.
- **Alert titles read `HOSTNAME // Event`**, hostname uppercased. No em-dashes anywhere user-facing.

## Status

v1.1.1, in production on a fleet of 20+ Windows Server hosts running IIS workloads, per the README. History shows the arc:

| Date (2026) | Version | Milestone |
|---|---|---|
| Jun 15 | 0.0.30 | Earliest commit in this repo (earlier history not imported). Helm chart and docker-compose install. |
| Jul 4 to 7 | 0.0.32 to 0.0.43 | Uptime monitor, log download, agent self-update, health checks moved into applications, blackbox exporter, alerting rework |
| Jul 8 | 1.0.0 | v1 |
| Jul 26 to Aug 22 | 1.0.1 to 1.0.6 | Security fixes, bug fixes, UI polish |
| Aug 30 | 1.0.5 to 1.0.9 | Incident management, Settings page |
| Oct 12 | 1.0.12 to 1.1.1 | SQL Server, failover cluster and AG support; SQL UI rebuild |

Cadence is bursty: intense multi-day pushes, then quiet weeks. One engineer.

## Direction

The stated theme for v2 is **incident response**: close the loop from "something is wrong" to "it was fixed, here is what happened". `ROADMAP.md` orders the work. Its current state against the code:

| ROADMAP item | Reality now |
|---|---|
| 1. Auth on account/organization endpoints | Done: both are public only for first-run bootstrap, gated in the handler |
| 1. Rate limiting on login/MFA | Done for `/auth/login` and `/auth/mfa` (10 per 5 min per IP). `/mfa/totp-uri` and `/mfa/confirm` accept username+password and are not limited. |
| 1. Lock down CORS | Done: `WEBURL` plus `localhost:5173` only |
| 1. Agent token rotation | Open. Tokens still expire in 100 years. |
| 2. Test coverage | Partly done: 46 Go tests on incidents, settings, machine reassignment, SQL. Hub, auth and heartbeat sync (the roadmap's top three) are still untested. No frontend tests. |
| 3. Incident recovery workflow engine | Open. Incidents exist now, which is the data the engine needs. Seam is in place. |
| 4. w3wp.exe process visibility | Open |
| 5. Windows Event Log, IIS ARR, per-URL analytics | Open (Event Log already flows to Loki, unused) |
| 6. Audit trail | Open |

Ideas in the owner's scratch file (`todo`, gitignored) that are not in the roadmap: making the platform generic beyond Windows (Linux agent; the `os_type` field anticipates it), Windows Event Forwarding and Palantir's alerting-detection-strategy framework as references, MinIO for memory dumps and log downloads, log retention on hosts, full agent uninstall cleanup, and a security audit of what account the agent runs as.

What we stand to gain from the recovery phase: today WIMP shortens time-to-know and time-to-act. Recovery shortens time-to-fix to zero for the known failure classes (crashed app pool, hung worker process, full disk), and the incident timeline turns "we got paged" into an artefact a team can review.
