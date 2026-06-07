# su3i

**Windows Infrastructure Management Platform — MVP**

---

## What It Is

su3i is an internal platform for managing pools of Windows/IIS application instances across multiple machines. It replaces the need to RDP into individual servers for routine operational tasks, and provides a unified view of application health, metrics, and logs across a horizontally scaled fleet.

It is not a deployment tool. It is not a cloud orchestration platform. It does one thing: give operators full operational visibility and control over legacy IIS workloads running on Windows servers, from a single browser tab.

---

## Problem It Solves

Managing 10–100+ Windows servers today means:

- RDP sessions into individual machines for every operational task
- No unified view of which apps are healthy across which machines
- No central log access — each machine has its own log files
- No metrics aggregation — you find out something is wrong when users complain
- Restarting an app pool across 20 machines is a manual, error-prone process

su3i eliminates all of this.

---

## Architecture

```
Browser (React)
      │
Control Plane (Go + PostgreSQL)
      │
      │  WebSocket (agent dials OUT)
      │
┌─────┴────┬──────────┬──────────┐
Agent     Agent     Agent     Agent
WEB01     WEB02     WEB03     WEB04
  │
  ├── windows_exporter → Prometheus → Grafana
  └── Fluent Bit → Loki
```

**No inbound firewall rules required on machines. Agents dial out.**

---

## Components

### Agent (Go — Windows Service)

Installed on every managed machine. Single self-contained `.exe`, no runtime dependencies. Responsibilities:

- Dials out to control plane via authenticated WebSocket on startup
- Sends heartbeat every 30 seconds
- Auto-discovers IIS sites and app pools via PowerShell on first connect
- Executes IIS control commands received from control plane
- Reports current IIS state on demand

### Control Plane (Go)

Central API server. Responsibilities:

- Machine registry (add machine → generate token → agent connects → machine appears)
- Maintains WebSocket connections to all agents
- Routes commands from UI to the correct agent(s)
- Stores project/application/machine hierarchy in PostgreSQL
- Proxies Loki log queries to the frontend
- Serves the React frontend

### Bootstrap Script (PowerShell)

One command run on each machine:

```powershell
iex (iwr "https://su3i-host/bootstrap?token=abc123").Content
```

Installs and starts three Windows Services:

1. `windows_exporter` (metrics)
2. `Fluent Bit` (log shipping → Loki)
3. `su3i-agent` (control plane connection)

Machine appears in dashboard within 60 seconds of running this command.

### Frontend (React + Tailwind)

Single-page application. Three primary views: Projects, Application, Instance.

---

## Data Model

```
Project
  └── Application
        └── Instance → Machine
```

One machine can host multiple applications. One application spans multiple machines (instances). A project groups related applications.

---

## Features (MVP)

### Machine Management

- Add machine by IP or hostname
- Generate per-machine bootstrap token
- Online/offline status via heartbeat
- Auto-discovery of IIS sites and app pools on agent connect
- Map discovered apps to project applications

### Hot Actions

Per instance:

- Start / Stop IIS Site
- Start / Stop / Recycle App Pool

Per application (rolling across all instances):

- Restart All
- Stop All
- Start All
- Recycle All

### Observability

- Grafana dashboard embedded per machine and per application (metrics via windows_exporter + Prometheus)
- Live log tail per instance via Loki query (Fluent Bit ships logs with machine/app labels)
- Health status aggregated per application: N/M Healthy

### Auth

- Single admin user
- Username and password generated at deployment time
- JWT session tokens
- No onboarding flow — credentials printed to stdout on first boot

---

## Dependencies

| Component        | Technology                              |
| ---------------- | --------------------------------------- |
| Control Plane    | Go                                      |
| Agent            | Go (Windows Service)                    |
| Database         | PostgreSQL                              |
| Migrations       | golang-migrate                          |
| Frontend         | React + Tailwind CSS                    |
| Metrics pipeline | windows_exporter + Prometheus + Grafana |
| Log pipeline     | Fluent Bit + Loki                       |
| Agent transport  | WebSocket over HTTPS                    |

---

## What Is Explicitly Out of Scope (MVP)

- Deployment / release management
- File browser
- Shell / terminal access
- Rollbacks
- Approval workflows
- Multi-tenancy / RBAC
- Notifications (Slack, email, etc.)
- Scheduled actions
- Blue/green or canary strategies

---

## 2-Week Build Plan

### Week 1 — Agent + Control Plane Core

**Days 1–2: Shared protocol + agent skeleton**

- Define shared Go types: Command, Response, DiscoveryResult, Site, AppPool
- Agent Windows Service skeleton (start/stop/install)
- WebSocket dial-out from agent to control plane
- Registration handshake with token validation

**Days 3–4: IIS operations**

- PowerShell execution wrapper in Go
- Discover: `Get-Website`, `Get-WebAppPool` → parse JSON output
- Actions: start/stop site, start/stop/recycle pool via PowerShell
- Agent command dispatch loop

**Day 5: Control plane foundations**

- PostgreSQL schema: projects, applications, machines, instances
- Machine registration API
- WebSocket agent connection registry
- Token generation endpoint

### Week 2 — Frontend + Observability + Polish

**Days 6–7: Frontend core**

- Login screen
- Project → Application → Instance navigation
- Health status display (online/offline, N/M healthy)
- Action buttons with confirmation

**Days 8–9: Observability wiring**

- Grafana iframe embed per machine/application
- Loki log tail component (poll or SSE)
- Fluent Bit config template in bootstrap script
- Bootstrap script generator (token injection + all 3 services)

**Day 10: End-to-end testing + hardening**

- Full flow test on a clean Windows Server VM
- Agent reconnection on disconnect
- Rolling action sequencing (wait for health before next instance)
- README + deployment instructions
