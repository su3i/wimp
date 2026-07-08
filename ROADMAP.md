### Roadmap

v1 covers the core loop: agents report in, machines/app pools/sites are visible and controllable, metrics flow through Prometheus, logs flow through Loki, and alerts flow through Alertmanager with a severity system (Info/Warning/Critical/Sev/Disabled) and per-type configurability. This document lays out what v2 builds on top of that.

Themes are ordered by what should land first, not by size - security fixes are small but sequenced ahead of larger feature work because they're gaps in something already shipping.

---

### 1. Security hardening

v1 shipped with a few known gaps that are fine for a controlled/internal rollout but need closing before any wider or public-facing deployment.

- **Auth on account/organization endpoints.** `POST/PUT /account` and `POST/PUT /organization` are currently reachable without a token - anyone who can reach the control plane can create accounts or enumerate users by username. Needs `AuthMiddleware` at minimum; likely also needs a "first admin only" bootstrap gate so account creation isn't open-ended after initial setup.
- **Rate limiting on `/auth/login` and `/auth/mfa`.** Both are wide open to brute force today. A Redis-backed counter (cache infra already exists) or `gin-contrib/ratelimit` closes this.
- **Lock down CORS.** `AllowAllOrigins: true` with `AllowHeaders: "*"` in production - restrict to the known web origin.
- **Agent token rotation.** Tokens get a 100-year expiry on first connect, meaning a compromised agent token is effectively permanent. Needs a rotate-token endpoint (and probably a "last rotated" surface in the Hosts UI) rather than relying on expiry.

### 2. Test coverage

There is no test coverage anywhere in the repo today - zero Go test files, zero frontend tests. This isn't a "someday" item once the agent protocol, alert routing, and auth flows are the things holding customer trust. Priority order:

1. Hub command dispatch (`internal/hub`) - this is the one piece of concurrent, stateful code in the whole system; the per-connection write-mutex fix (already landed) is exactly the kind of thing a regression test should pin down permanently.
2. Auth service (login, MFA challenge, token refresh/revoke) - highest blast radius if broken.
3. Heartbeat/sync logic (app pool + site state reconciliation) - silent drift here is invisible until someone notices stale state in the UI.

Frontend: no framework is wired up yet (no Vitest/Testing Library in `package.json`). Worth deciding this early since it shapes how new components get written going forward.

### 3. Incident recovery workflow

The Alerts Rework (v1) deliberately stopped short of building recovery - it added exactly one seam: `internal/application/recovery/recovery.go`, called once per Sev-level alert, currently a no-op log line. v2 is where that seam gets a real engine behind it. This was scoped out on purpose ("not a 1:1 alert-type mapping... a multi-step workflow") rather than half-built, so the design work is still open:

- Define what a workflow actually looks like: try X, check, try Y if X didn't resolve it. Needs its own state machine, not a switch statement.
- **Auto dumps.** On a Sev-level app pool/process incident, take a memory dump automatically, make it downloadable from the incident view.
- **Self-healing actions.** Auto-restart an app pool or machine as a workflow step, with guardrails (don't restart-loop something that's crash-looping for an unrelated reason).
- **http.sys unstick.** For hung worker processes, drain active requests and restart the backend without dropping the whole site.
- **Incident timeline.** A per-incident view stitching together the triggering alert, every recovery step attempted, and the eventual resolution (or escalation) - this is what turns "we got paged" into "here's what happened and what we did about it."

### 4. w3wp.exe process-level visibility

App pool state today is taken entirely from IIS's own self-report - nothing checks whether there's a live worker process actually backing a pool that shows "Started". This was explicitly cut from the Alerts Rework to avoid shipping a heuristic that cries wolf on idle-but-healthy pools. Revisiting this means solving the idle-vs-crashed distinction properly (likely: process existence + a liveness signal, not just existence) before it becomes a new alert type.

Related, same problem space: specific app-pool-level metrics (worker process memory/handle count/thread count over time) tied to the w3wp relationship, which is currently invisible.

### 5. Deeper Windows/IIS integration

- **Windows Event Log.** No integration today - currently the only signal from a host is metrics (Prometheus) and IIS/app logs (Loki via fluent-bit). Application and System event log entries (crashes, service failures, .NET unhandled exceptions) are a real gap.
- **IIS ARR (Application Request Routing).** No visibility or control today for hosts using ARR as a reverse proxy/load balancer layer.
- **Per-URL analytics.** Which URLs are throwing the most errors, which are slowest to respond. This needs either IIS log parsing (the raw data already flows through fluent-bit → Loki) or request-level instrumentation - worth spiking both before committing to one.

### 6. Audit trail

RBAC and TOTP MFA already exist (per-role authorization checks are wired through every handler; `/mfa/totp-uri` + `/mfa/confirm` + `EnforceMfa` config are live). What's missing is a record of *who did what* - every machine command, app pool action, alert config change, and user/role change should be attributable and queryable. This is a straightforward append-only log table + a UI view, but touches every mutating handler, so it's worth doing as a deliberate pass rather than bolting on per-feature.
