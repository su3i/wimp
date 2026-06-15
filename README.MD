### Windows Infrastructure Management Platform

WIMP is a self-hosted platform for full operational visibility and control over windows server fleets running IIS workloads.

---

### The Problem

Something goes wrong in production. A sysadmin needs to know what happened, on which machine, and why, in seconds. Today that means opening an RDP session to check logs, opening another to check the process, and a third to restart the app pool. Multiply that across 10, 50, or 100 servers and the problem is obvious.

WIMP puts everything in one place. Live metrics per machine, log streams queryable in the browser, critical alerts routed to wherever the team lives, and full IIS control without touching a terminal. Start an app pool, stop a site, recycle across every instance of an application at once. All from a single tab.

---

### Architecture

Agents run on each Windows machine and dial out to the control plane over WebSocket. No inbound firewall rules required on managed machines. The control plane runs in Kubernetes or Docker and the browser communicates only with the control plane.

```
                        ┌──────────────────────────────────────────────┐
                        │              Web App  (React SPA)            │
                        └─────────────────────────┬────────────────────┘
                                                  │  HTTP
                                                  ▼
  ┌──────────┐     ┌────────────────────────────────────────────────────────────────┐
  │ Postgres │◄────│                     Control Plane  (Go)                        │
  └──────────┘     │                                                                │
                   │   Machine Registry  ·  WebSocket Hub  ·  Alert Engine          ├──► Alertmanager
  ┌──────────┐     │                                                                │
  │   Loki   │◄────┤  log queries                                                   │
  └─────▲────┘     └─────────────────────────────────────────────────┬──────────────┘
        │                                                            │  WebSocket
        │  log ingestion                                             │  (agents dial out)
        │                                                            ▼
        │                                     ┌──────────────────────────────────────────┐
        │                                     │          Windows Machines  (× N)         │
        │                                     │                                          │
        └────────── Fluent Bit ───────────────┤  WIMP Agent ──────────────────────► IIS  │
                                              │  Fluent Bit                              │
  Prometheus ──────── scrape ────────────────►│  windows_exporter                        │
       ▲                                      └──────────────────────────────────────────┘
       │  HTTP SD  /prometheus/targets
  Control Plane
```

---

### Components

**Control Plane:** Go HTTP API. Manages the machine registry, holds WebSocket connections to all agents, routes commands, stores the project/application/machine hierarchy in Postgres, and proxies Loki queries to the frontend.

**Agent:** Single Go binary installed as a Windows service. Dials out to the control plane on startup, sends a heartbeat every 30 seconds, auto-discovers IIS sites and app pools, and executes commands received from the control plane.

**Bootstrap Script:** One PowerShell command per machine. Downloads and installs windows_exporter, Fluent Bit, and the WIMP agent as Windows services. The machine appears in the dashboard within 60 seconds.

**Web:** React SPA.

---

### Features

**Machine Management**

- Add machines by IP or hostname and generate per-machine bootstrap tokens.
- Online, offline, and deleting status tracked via heartbeat.
- Two-phase deletion: returns the uninstall command and hard deletes on agent disconnect.
- Auto-discovery of IIS sites, app pools, bindings, and physical paths on connect.

**IIS Control**

- Start, stop, restart, and recycle per app pool or site.
- Rolling operations across all instances of an application: restart all, stop all, start all.
- Pending state cache: command status persists across browser refreshes while the action completes.

**Observability**

- Per-machine and per-application metrics via windows_exporter and Prometheus, surfaced directly in the UI.
- Log streams per machine via Fluent Bit and Loki, queryable in the browser.
- Critical alerts routed through Alertmanager to Telegram or any configured receiver.
- HTTP service discovery endpoint for Prometheus at `/prometheus/targets`.

**Auth**

- JWT session tokens with optional MFA enforcement.
- Default admin credentials set at deploy time.

---

### Todo

- Multi-user support and RBAC.
- Deployment and release management.
- Scheduled actions and approval workflows.
- Shell and terminal access.
- File browser.
- Blue/green and canary deployment strategies.
- Email and Slack notifications.

---

### Deployment

**Kubernetes (Helm)**

```sh
helm upgrade --install wimp ./helm -n wimp --create-namespace -f your-values.yaml
```

Bundles Loki, Prometheus, and Alertmanager as optional subcharts.

**Docker (Compose)**

```sh
cp .env.example .env
docker compose up -d
docker compose --profile observability up -d
```

Core only by default. The observability profile adds Loki, Prometheus, Alertmanager, and Grafana.

---

### Stack

|               |                               |
| ------------- | ----------------------------- |
| Control Plane | Go                            |
| Agent         | Go (Windows Service)          |
| Database      | Postgres / SQLite             |
| Frontend      | React + Tailwind              |
| Metrics       | windows_exporter + Prometheus |
| Logs          | Fluent Bit + Loki             |
| Alerts        | Alertmanager                  |
| Transport     | WebSocket over HTTPS          |
| Kubernetes    | Helm                          |
