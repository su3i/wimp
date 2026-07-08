<div align="center">
  <img src="web/public/favicon.svg" width="120" height="120" alt="WIMP logo" />

  # WIMP

  ### Windows Infrastructure Management Platform

  Self-hosted monitoring, IIS control, and alerting for Windows server fleets. Built for the moment something breaks at 2am and you need to know what, where, and why in seconds.

  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE.md)
</div>

---

### What exactly is WIMP?

WIMP is a self-hosted platform for monitoring, managing, and responding to incidents on Windows Server fleets running IIS. Most teams end up hand-rolling this layer themselves out of RDP sessions, PowerShell one-liners, and a spreadsheet tracking which machine runs what. WIMP replaces all of that with one live dashboard that works the same whether it's watching three servers or three hundred.

A lightweight agent runs on every Windows machine and reports in. From there, WIMP gives you live metrics per host, searchable logs, alerts that reach you before a customer notices, direct control over every IIS site and app pool (start, stop, restart, recycle), or roll a restart across an entire application's worth of instances, all from a browser tab without opening a single RDP session.

---

### The problem

Something goes wrong in production. Someone needs to know what happened, on which machine, and why, in seconds. Today that usually means opening an RDP session to check the process, check the logs, and possibly restart the app pool by hand. Multiply that across ten, no - fifty, no - a few hundred servers and the problem is obvious: the MTTR is bounded by how fast a human can hop between machines.

WIMP collapses all of that into one place. Live metrics per machine, log streams queryable in the browser, alerts routed to wherever the team actually lives, and full IIS control without touching a terminal. Start an app pool, stop a site, roll a restart across every instance of an application, all from a single tab.

---

### How it works

Every Windows machine runs a single, small Go binary as a Windows service. That agent dials *out* to the control plane over a WebSocket connection on startup and stays connected. Nothing needs to be reachable on the machine itself, so there are no inbound firewall rules to open and nothing exposed to the network the way an RDP or WinRM endpoint would be. The agent sends a heartbeat, auto-discovers every IIS site and app pool on the box, ships logs, and executes whatever commands the control plane sends it.

The control plane is a Go HTTP API that holds those WebSocket connections, keeps the machine > application > project hierarchy in Postgres (or SQLite for smaller setups), and is the only thing the browser ever talks to. Metrics flow through Prometheus via `windows_exporter` running alongside the agent. Logs flow through `fluent-bit` into Loki. Alerts get evaluated centrally and pushed out through Alertmanager to the receiver your team already uses.

A one-line PowerShell bootstrap command is all it takes to bring a new machine online. The agent, `windows_exporter`, and `fluent-bit` install themselves as Windows services, and the machine shows up in the dashboard in seconds.

---

### Features

**Fleet management**

- Add a host by hostname, get back a one-line PowerShell bootstrap command, done.
- IIS sites, app pools, bindings, and physical paths auto-discover on connect, nothing to register by hand.
- Live status per host (online, offline, pending) via a 30-second heartbeat, with hostname, IP addresses, Windows version, and agent version all visible at a glance.
- Remote shutdown and restart from the browser, with a confirmation step in front of both.
- Fleet-wide agent self-update: bump the target version once, every machine updates itself on its next connect. No manual re-install across a fleet of any size.

**IIS control**

- Start, stop, restart, and recycle any app pool or site directly from the web interface.
- Rolling restarts across an entire application: pools restart one at a time with a fixed delay between each, not all at once, so a multi-instance application never drops to zero healthy capacity mid-restart.

**Applications**

- Group the app pools that make up one logical product across one or more hosts, and see aggregated health (e.g. "5/5 healthy") without cross-referencing five separate host pages.
- Optional HTTP health checks per application, configurable interval, feeding straight into the same alert pipeline as every other event in the system.
- A log viewer scoped to the whole application: pick a host and app pool, search, scroll to load more, expand to a full-screen view. Download as a zip or clear logs entirely, with a confirmation step before anything gets deleted.

**Metrics**

- A dashboard that leads with the numbers that actually matter day to day: request throughput, bandwidth in/out, capacity overview, recent activity, etc.
- Per-host CPU, memory, disk, and network, scraped by `windows_exporter` and queried live from Prometheus, rendered directly in the app.
- Per-application aggregate views and a fleet-wide host CPU comparison chart, so a spike is visible against the rest of the fleet, not just in isolation.

**Logs**

- IIS and application logs ship through Fluent Bit into Loki and are queryable from the browser. The actual point of this feature is that nobody should need an RDP session just to grep a log file.
- Filter by host and app pool, most recent entries first, sane defaults.
- Download logs as a zip or clear them outright.

**Uptime monitoring**

- Independent HTTP and domain-based uptime checks, decoupled from any single host, for endpoints that live behind a load balancer or a DNS record rather than one specific machine.

**Alerting**

This is the deepest part of the platform, and it's had the most engineering attention.

- Five-tier severity system: Info, Warning, Critical, Sev, Disabled. Disabled means the event is never even recorded, not just silenced. Sev is reserved for the alerts that mean drop everything.
- Every alert-worthy event in the system funnels through a single registry: machine connect/disconnect, planned shutdown/restart, fleet-wide agent updates, app pool and site start/stop, sidecar health (`windows_exporter`, Fluent Bit), HTTP health check up/down, and high CPU, high memory, and low disk, each with a matching "recovered" alert.
- Every alert type's severity, and every numeric threshold behind the metric-based alerts, is configurable per deployment via environment variables, with sensible defaults out of the box.
- A sustained Sev-level incident gets a repeat reminder every 15 minutes instead of going silent until it resolves. The dashboard still counts it as one incident, not a wall of duplicate notifications, so the numbers stay honest.
- Alertmanager grouping is keyed by machine instance, not just alert name, so ten machines alerting at once produce ten separate, correctly-grouped notifications instead of nine of them waiting behind Alertmanager's group window for the first one to resolve.
- Outbound alert messages are built to be scanned in a hurry: a severity glyph, an always-uppercase hostname, and a link straight back to the exact machine in WIMP, not a wall of raw label text.
- A dedicated, deliberately minimal incident-recovery hook fires on every Sev-level alert today. It doesn't do anything on its own yet, and that's intentional. It exists so the next phase of this project, automatic recovery workflows, has a stable, obvious place to attach to instead of getting bolted onto the alert pipeline after the fact. See [Roadmap](#roadmap).

**Access control**

- Role-based authorization enforced on every API route, not just hidden in the UI.
- TOTP-based multi-factor authentication, optionally enforced platform-wide.
- Multi-project isolation: hosts, applications, alerts, and activity are all scoped to a project, so one deployment can cleanly serve more than one team or environment.

---

### Tech stack

|                | |
| -------------- | ------------------------------ |
| Control plane  | Go |
| Agent          | Go, packaged as a Windows service |
| Database       | Postgres / SQLite |
| Frontend       | React + TypeScript + Tailwind |
| Metrics        | `windows_exporter` + Prometheus |
| Logs           | `fluent-bit` + Loki |
| Alerting       | Alertmanager |
| Transport      | WebSocket (agents dial out; nothing inbound to open) |
| Deployment     | Kubernetes via Helm |

---

### Getting started

**Kubernetes (Helm)**

```sh
helm upgrade --install wimp oci://ghcr.io/su3i/charts/wimp -n wimp --create-namespace -f your-values.yaml
```

No repo checkout needed, the chart is published straight from GHCR on every change to `helm/`. Pin an exact release with `--version x.y.z` if you don't want to track the latest. Bundles Loki, Prometheus, and Alertmanager as optional subcharts. Bring your own observability stack, or let the chart stand one up for you.

---

### Project status

WIMP is at v1. It's not a prototype: it's running in production today, managing a real fleet of over 20 Windows Server hosts across live IIS workloads, and it's had the kind of iterative hardening that only comes from actually depending on something, including fixed alert-delivery races, tightened WebSocket reconnect behavior, closed authorization gaps, and a full rework of the alerting pipeline to get severity, delivery, and noise right.

It's also open source. If you're managing Windows Server and IIS infrastructure and doing it the hard way, RDP sessions and PowerShell scripts stitched together by hand, this is built to replace that.

---

### Roadmap

v1 covers the core loop: visibility, control, and alerting. The direction from here is **incident response**: not just telling you something is wrong, but helping close the loop on it.

The alerting rework that shipped in v1 was built with this in mind: the five-tier severity system exists so a Sev-level event can be treated differently from noise, and every Sev alert already fires through a dedicated recovery hook. The next phase builds a real workflow engine behind that hook: automatic diagnostic dumps on a Sev-level incident, self-healing actions like restarting a crashed app pool automatically, unsticking a hung worker process without dropping the whole site, and a timeline view that ties an incident's trigger, response, and resolution together in one place.

Alongside that: closing a handful of known security gaps before wider deployment (public account endpoints, rate limiting, agent token rotation), building out real test coverage, deeper Windows Event Log and IIS ARR integration, and an audit trail across every mutating action.

The full, current list lives in [`ROADMAP.md`](./ROADMAP.md).

---

### License

MIT. See [`LICENSE.md`](./LICENSE.md).
