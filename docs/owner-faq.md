# Owner FAQ

Questions people ask about WIMP, answered the way the owner answers them. The answers are grounded in the code and the other docs here; where an answer is the owner's judgement rather than a fact in the repo, it reads as an opinion.

## Product

**What is WIMP, in one sentence?**
A self-hosted control plane for Windows Server fleets running IIS and SQL Server: live status, logs, metrics and alerts, plus the ability to act (restart a pool, roll an application, fail over SQL) from one browser tab instead of RDP.

**Who is it for?**
Teams running .NET on IIS, often with SQL Server, on anything from a handful to a few hundred Windows servers, who either cannot send telemetry to a SaaS or do not want per-host pricing. It was built for a real production fleet of 20+ hosts first, and features arrive in the order that fleet needs them.

**Why not just use Datadog / Grafana / SCOM?**
Grafana and Prometheus are inside WIMP; they show you data but cannot restart an app pool and know nothing about IIS sites or SQL failover cluster instances. SaaS APMs are per-host priced and still do not give you control actions. SCOM can do it but is a project to run. WIMP is one Helm install and one PowerShell line per host.

**Is it a business?**
It is MIT-licensed and open source. There is no billing, licensing or SaaS code. Multi-project support exists so one deployment can serve several teams or environments.

**What's next?**
Incident response. Today WIMP tells you fast and lets you act fast. Next it should act for you on the failure classes we understand: restart a crashed pool, take a memory dump on a Sev incident, unstick a hung worker without dropping the site, and keep a timeline of what was tried. The recovery hook is already called on every Sev alert, and incidents already exist as data, which is what the engine needs. Before that, close the agent-token-rotation gap and add an audit trail.

**Why is there no mobile view?**
Dense tables and log views do not work on a phone, and a half-working responsive layout is worse than an honest "desktop only" screen. Alerts reach phones through Alertmanager receivers.

## Architecture

**Why does the agent connect out instead of the server connecting in?**
So hosts expose nothing for control. No WinRM, no open management port, no per-host credentials stored centrally, works across NAT. The connection doubles as presence: if the socket is open the host is online.

**But Prometheus scrapes the host on 9182?**
Yes. "Nothing inbound" is true for control, not metrics. windows_exporter is a standard scrape target and bootstrap opens the firewall for it. If a host cannot allow that, the fix is to have the agent push metrics, not to change the control channel.

**Why only one control plane replica? Isn't that a single point of failure?**
It is, deliberately. Agent sockets, pending commands, alert edge state and the incident lock are all in process. That trades HA for having no message bus, no distributed locks and no Redis requirement, which is the right trade for one engineer at this fleet size. A restart costs a few seconds of agent reconnects and nothing is lost, because the DB holds the state of record and the agents re-report everything on connect. The path to more replicas is known and written down (decisions.md, entry 9).

**Why Postgres and SQLite?**
SQLite for running it on a laptop or a tiny install with zero dependencies; Postgres for production. GORM gives both from the same models. The cost is two near-identical repository folders, which is accepted for now.

**Why Go?**
One static `agent.exe` with nothing to install on the host, cross-compiled in CI, and the same protocol package compiled into both sides so they cannot drift.

**Why Alertmanager instead of sending to Telegram/Slack directly?**
Alertmanager already does routing, grouping, silencing and every receiver. WIMP's job is to produce well-labelled alerts. There is an old Telegram client in the code from before that decision; it is unused.

**Why do alerts not fire again while something stays broken?**
They are edge-triggered on purpose: fire on the transition, recover on the reverse. Sustained Sev metric breaches get a reminder every 15 minutes. Anything else stays open as an incident in the UI.

**Why five severities?**
Critical gets diluted into "look at it today". Sev is the "drop everything" tier and is what triggers recovery. Disabled is an explicit opt-out: the event is never recorded, but if a disabled type is a recovery it still closes its incident.

**How do incidents work?**
Every fault alert has a recovery partner. An incident opens on the first and closes on the second, keyed by project, machine, kind and subject, so two failing pools on one host are two incidents. Nothing extra to configure.

## Behaviour people ask about

**Why can't I shut a server down from WIMP?**
Because a host shut down remotely stays down until someone physically turns it on. Same reason SQL Server cannot be stopped from the browser. Restart is available.

**I restarted a host and nobody got paged. Is that a bug?**
No. A restart you asked for is announced as Info. If the host does not come back, the disconnect alerting still applies. Same for SQL start/restart: silent if it comes back within 5 minutes, alerts if not.

**Why did a planned AG failover refuse?**
The target replica must be synchronous-commit and every database on it SYNCHRONIZED. Anything else risks data loss, so WIMP will not do it from a button.

**Why does a clustered SQL instance's incident close on a different node?**
Because the instance belongs to the cluster, not to a node. It is tracked as `fci:<virtual name>\<instance>`, so a failover closes the incident that opened on the old owner.

**Why is the health check timeout so long?**
A probe that times out looks exactly like a refused connection, so a tight timeout reports slow sites as down. Slowness has its own alert at 5 s; down means it did not answer within 30 s, twice.

**What happens when I bump the agent version?**
With auto-update on, every host updates itself the next time it connects, through a scheduled task that stops the service, swaps the binary and starts it. With it off, update hosts individually from the host page. Release the agent to GitHub before changing the version anywhere.

**A host shows "reassigned". What happened?**
Bootstrap was run again on that machine, usually from another project. The old row is retired, removed from Prometheus, and points at the new one. It will not come back online.

**Where are the logs?**
Only for app pools that are in an application and have a log directory set. Fluent Bit tails that directory into Loki; the application page reads them back. Windows Event Log Application/System also flows to Loki but is not surfaced in the UI yet.

## Engineering practice

**What's the test story?**
46 Go tests covering the logic that has bitten us: incident pairing, settings precedence, host reassignment, SQL scope and edge rules. The WebSocket hub, auth and heartbeat reconciliation are the next to cover. There are no frontend tests and no CI test gate yet. That is a known gap, not a philosophy.

**How do you release?**
`scripts/bump-app.sh` or `scripts/bump-agent.sh`, push to main. The bump edits the workflow file, which is the trigger. Images go to Docker Hub, the agent to GitHub releases, the chart to GHCR.

**Coding style?**
Comments explain why, at the line where it matters, including what went wrong before. Deliberate shortcuts are marked `ponytail:` with their ceiling and upgrade path. Standard library before dependencies. No em-dashes anywhere, alert titles use `HOST // Event`. New screens copy the existing Hosts/Applications layouts rather than inventing new ones.

**What would you change if you started over?**
(Owner judgement.) One repository tree instead of two copies for Postgres and SQLite, a real migration tool, project-scoped RBAC from day one, rotation built into agent tokens, and rolling restarts as a server-side job rather than a browser loop.

**What are you least comfortable with today?**
(Owner judgement.) Agent tokens that never expire, Prometheus reachable from browsers without auth, RBAC that only really works for superadmins, and the host-delete path that a heartbeat can undo (operations.md, B1).
