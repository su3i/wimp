# WIMP: engineering handbook

These docs explain WIMP well enough that someone who has never seen the code can speak for it as its owner: what it is, who it is for, how it is built, why it is built that way, what it costs, and where it is going.

They were written on 2026-10-20 against commit `147e327` (app 1.1.1, agent 1.0.4, Helm chart 0.0.22). Where the code disagrees with the top-level `README.MD` or `ROADMAP.md`, these docs follow the code and say so explicitly.

## Reading order

| File | Read it for | Time |
|---|---|---|
| [product.md](product.md) | What WIMP is, the problem, who uses it, feature inventory, status, direction | 10 min |
| [architecture.md](architecture.md) | Every component, every data flow, where state lives, the wire protocol, the data model | 25 min |
| [codebase.md](codebase.md) | Directory map, layering, conventions, recipes for common changes, build and release | 20 min |
| [decisions.md](decisions.md) | Why X and not Y for every significant choice, what it buys, what it costs, when to revisit | 25 min |
| [operations.md](operations.md) | Deploying, configuring, onboarding hosts, security posture, known bugs and gaps | 15 min |
| [owner-faq.md](owner-faq.md) | The questions people ask the owner, answered the way the owner answers them | 10 min |

## WIMP in one paragraph

WIMP (Windows Infrastructure Management Platform) is a self-hosted, MIT-licensed control plane for fleets of Windows Servers running IIS and SQL Server. A small Go agent runs as a Windows service on every host and dials out to the control plane over a WebSocket, so nothing on the host has to accept inbound connections. Through that one connection the control plane discovers what each host runs (IIS app pools and sites, SQL Server instances, failover clusters, availability groups), receives a heartbeat every 30 seconds, and sends commands back (start, stop, recycle app pools, restart hosts, fail over SQL). Metrics go through `windows_exporter` into Prometheus, logs go through Fluent Bit into Loki, and every alert-worthy event funnels through a single function that records it, groups it into an incident, and forwards it to Alertmanager. A React SPA sits on top. It runs in production on a fleet of 20+ Windows hosts. The next phase is automated incident recovery, and the hook for it already exists.

## Numbers worth memorising

| | |
|---|---|
| Source size | ~30k lines (Go ~17k including tests, TypeScript/TSX ~12k), plus ~0.3k PowerShell |
| Go | 1.25, Gin, GORM, Casbin, gorilla/websocket, kardianos/service, go-mssqldb |
| Web | React 19, Vite 8, TypeScript 6, Tailwind 4, TanStack Query 5, Zustand 5, Recharts 3 |
| Sidecars pinned by bootstrap | windows_exporter 0.31.7, Fluent Bit 5.0.7 |
| Heartbeat | 30 s from agent; control plane drops the connection after 90 s of silence |
| Command timeout | 30 s (HTTP request blocks waiting for the agent's reply) |
| Checkers | Metrics thresholds every 60 s, SQL DMV alerts every 60 s, health checks every 30 s, all from Prometheus |
| Alert types | 43 in the registry, 5 severity levels |
| Replicas | Control plane is designed for exactly one (see decisions.md) |
| Tests | 46 Go tests across 9 files, all passing; zero frontend tests |
| History | 100 commits from v0.0.30 (2026-06-15) to v1.1.1 (2026-10-12); v1.0.0 on 2026-07-08 |

## Glossary

| Term | Meaning in this codebase |
|---|---|
| Control plane | The Go HTTP API in `cmd/app`. Also called "the app" or "the server". |
| Agent | The Go Windows service in `cmd/agent`. One per host. |
| Sidecars | `windows_exporter` and `fluent-bit`, installed by bootstrap next to the agent. Their health is reported in every heartbeat. |
| Host / machine | Same thing. The UI says Host, the code and DB say `machine`. Old `/machines` URLs redirect to `/hosts`. |
| Project | Tenant boundary. Hosts, applications, incidents and alerts belong to one project. Seeded `default` project cannot be deleted. |
| Application | A user-defined group of IIS app pools (and SQL instances) across hosts that make up one product. Owns the optional HTTP health check and log paths. |
| Role (host) | `iis`, `sql`, `cluster`: derived from discovery, never stored. Drives which nav entries appear. |
| Sev | The top severity tier, above Critical. "Drop everything." Fires the recovery hook. |
| Incident | A span from an "opens" alert to its matching "closes" alert, e.g. `app_pool_stopped` to `app_pool_started`. |
| Bootstrap | The one-line PowerShell install that turns a pending machine row into an online host. |
| Reassigned | Terminal machine status: the same physical box was re-bootstrapped into another row/project. |
| `ponytail:` comment | Marks a deliberate shortcut with a known ceiling and the upgrade path. Grep for them. |
