# Codebase guide

## Top-level layout

```
wimp/
├── cmd/
│   ├── app/main.go          control plane entrypoint (wiring, seeding, checkers, HTTP server)
│   └── agent/main.go        agent entrypoint (kardianos/service wrapper, --version flag)
├── internal/                all Go code; nothing is exported outside the module
│   ├── agent/               Windows agent: connection, IIS, SQL, Fluent Bit, update, sysinfo
│   ├── domain/              models, enums, repository interfaces, wire protocol, alert registry
│   ├── application/         services (business logic), one package per concept
│   ├── infrastructure/      Gin server + handlers, DB (postgres + sqlite), cache, Prometheus client
│   ├── hub/                 in-memory WebSocket registries (agents, browsers, pending commands)
│   ├── cache/               in-memory TTL maps for transient state (pending actions, sidecar health…)
│   └── config/              envconfig structs and global accessors
├── data/                    shipped inside the control plane image, read from ./data at runtime
│   ├── bootstrap.ps1        Go text/template rendered per machine
│   ├── cleanup.ps1          uninstall script
│   ├── model.conf, policy.csv   Casbin RBAC
├── web/                     React SPA (own Dockerfile, nginx, entrypoint)
├── helm/                    chart: control plane, web, optional Loki / kube-prometheus-stack / blackbox
├── scripts/                 bump-app.sh, bump-agent.sh, lib.sh (version bumping = release trigger)
├── .github/workflows/       agent release, control plane image, web image, chart publish
├── Dockerfile               control plane image
├── README.MD, ROADMAP.md    public face; partly stale, see product.md
└── todo                     owner's scratch list (gitignored pattern **/*todo, but present locally)
```

`docs/` is these files.

## Layering in `internal/`

```
handlers (infrastructure/server/handlers)
   │  parse HTTP, authorise with Casbin, call services, shape JSON
   ▼
services (application/*)
   │  business rules; take *config.DatabaseConfig; build repositories per call
   ▼
domain (domain/*)              ◄── models, enums, repo interfaces, protocol, alert registry
   ▲                               (no dependencies on the layers above)
   │ implements
infrastructure/database/{postgres,sqlite}/repositories
```

Cross-cutting singletons are reached through package functions, not injected: `config.Common()`, `config.Database()`, `config.Alerts()`, `hub.Get()`, `hub.Clients()`, `cache.GetCache()`, `database.GetDB(cfg)`. Services accept `cfg *config.DatabaseConfig` and call `database.NewXRepository(cfg)` inside each function. This is not dependency injection, and it does not try to be. It keeps call sites short and makes the single-process model explicit.

Package naming: imports often alias services as `xxxService` and domains as `xxxDomain` because the same name (`machine`, `notification`, `incident`) exists in both layers.

## Backend packages worth knowing

| Package | What lives there |
|---|---|
| `domain/protocol` | Every WebSocket message type and payload. Shared by both binaries. Change here first. |
| `domain/notification/alert_types.go` | `AlertTypeRegistry` and `IncidentBindings`. The authoritative list of what can alert and what opens/closes incidents. |
| `application/notification` | `EmitAlert`, the only way to alert. `AlertTitle` formats `HOST // Event`. |
| `application/incident` | Open/close logic, timeline, manual resolve |
| `application/setting` | Override snapshot, typed accessors (`SeverityFor`, `ReceiverMinSeverity`, `AlertingEnabled`, `EnforceMfa`), `env.go` mapping alert type to env var by explicit switch |
| `application/metrics`, `application/sqlmetrics`, `application/monitor` | The three Prometheus-polling checkers. `monitor` is application health checks despite the name. |
| `application/sqlinstance` | SQL discovery sync, scope rules, state edge evaluation, planned-action window |
| `application/machine` | Machine creation, bootstrap command strings, reassignment, deletion, derived roles |
| `application/application` | Applications, membership, log paths, Fluent Bit config build/push, file listing fan-out |
| `application/recovery` | The no-op seam |
| `application/telegram` | Unused. Delivery moved to Alertmanager. |
| `infrastructure/server/router.go` | Every route in one file, with comments on why public ones are public |
| `infrastructure/server/handlers/ws.go` | Agent connection lifecycle and message dispatch. The busiest file in the system. |
| `hub` | Agent connections with per-connection write mutex, browser broadcast, pending command channels |
| `cache` (top-level) | Pending pool/site/machine actions, sidecar health, Sev repeat timers, staged downloads |
| `infrastructure/cache` | Key/value cache interface with memory and Redis implementations (tokens, challenges, rate limits) |

## Agent package

`internal/agent` compiles for Windows in production. `sysinfo_other.go` and build tags keep `go build ./...` and `go test ./...` working on macOS/Linux. Files:

| File | Role |
|---|---|
| `agent.go` | `service.Interface` Start/Stop, run loop, logger fallback |
| `config.go` | Reads `config.json` next to the executable (written by bootstrap) |
| `connection.go` | Reconnect loop, register, heartbeat, read loop, log zip/list/clear |
| `iis.go` | `appcmd.exe` discovery and commands |
| `sql.go`, `sql_windows.go` | Registry + cluster discovery, normalised states |
| `sqlmon.go` | DMV poller, textfile writer, live queries, connection strings |
| `sqlcmd.go` | SQL start/restart/move/failover via PowerShell |
| `fluent.go` | conf.d reconciliation and Fluent Bit restart |
| `health.go` | `sc.exe query` for sidecar health |
| `update.go` | Self-update via scheduled task |
| `version.go` | `Version = "dev"`, set by `-ldflags -X` at release |

## Database code

Two parallel repository trees, `infrastructure/database/postgres/repositories` and `.../sqlite/repositories`, implement the same interfaces. They are near-identical copies; the only real differences are SQL date functions (`date_trunc` vs `strftime`). Any repository change must be made in both. Tests live only on the SQLite side.

Migrations are GORM `AutoMigrate` at boot, listed in `postgres/main.go` and `sqlite/main.go`. AutoMigrate adds tables, columns and indexes; it does not drop or rename. A rename needs a hand-written step that does not exist yet.

## Handler pattern

Almost every handler is the same shape:

```go
allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c),
    authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
if err != nil || !allow { c.JSON(403, gin.H{"error": "forbidden"}); return }
// parse params, check ownership (machineService.BelongsToProject), call service
c.JSON(http.StatusOK, gin.H{"message": "success", "things": things, "total": total, ...})
```

Errors go back as `{"error": "..."}`. Lists paginate with `page` and `per_page` (`utils.ParsePageQuery`). Command handlers are generated by closures over the action (`AppPoolCommand("restart")`).

## Frontend

```
web/src/
├── main.tsx, App.tsx         theme injection, QueryClientProvider, router, toaster
├── router/index.tsx          lazy route per page; /machines redirects to /hosts
├── pages/                    one file per screen; detail pages are the largest (ApplicationDetail ~1.2k lines)
├── components/
│   ├── layout/               AppLayout, Sidebar (role-driven nav), Header, ProjectSwitcher
│   ├── ui/                   Button, Badge, Card, Modal, ConfirmModal, RowMenu, Pagination, Input, Hint
│   ├── machine/, sql/, application/, dashboard/, activity/, projects/
│   └── common/               ErrorBoundary, LoadingScreen, Spinner, MobileBlock
├── services/*.service.ts     thin axios wrappers per resource; prometheus.service uses fetch
├── store/                    Zustand: auth (tokens in localStorage), project (persisted), ui
├── lib/axios.ts              base URL from VITE_API_BASE_URL, bearer header, 401 refresh queue
├── lib/query-client.ts       staleTime 0, refetchOnMount always, no refetch on focus, retry 1
├── config/theme.ts           GitHub Primer dark palette, injected as CSS variables
├── types/index.ts            shared API types
└── utils/                    time formatting, SQL helpers, health query helpers, cooldown hook
```

Conventions:

- Server state is TanStack Query only. Pages poll with `refetchInterval` rather than relying on pushes. The one browser WebSocket (`/ws`, opened on the Dashboard) carries notifications.
- Query keys include the active project key, e.g. `['machines', projectKey]`, so switching project refetches everything. The Sidebar reuses the hosts query to derive roles, so role-driven nav costs no extra request.
- Charts call Prometheus from the browser with PromQL built in the component.
- Styling is Tailwind 4 with theme tokens (`bg-surface`, `text-ink-dim`, `border-rim`, `text-danger`…). Dark only. JetBrains Mono.
- `cn()` in `utils/cn.ts` joins class names (no clsx/tailwind-merge dependency).
- Toasts via `sonner`. Icons via `lucide-react`.
- Desktop only: `MobileBlock` covers the app below the `lg` breakpoint.
- `web/README.md` is untouched Vite template text.

## Code conventions

- **Comments explain why, at length.** Most non-obvious decisions are documented at the line where they bite, often including the failure that caused them. Read the comment before changing the code under it.
- **`ponytail:` comments** mark a deliberate shortcut with its ceiling and upgrade path (for example the per-heartbeat PowerShell cluster query, and in-process alert state that assumes one replica). `grep -rn "ponytail:" internal web/src` gives the debt ledger.
- **No em-dashes** in code, comments, UI strings, alert titles or commit messages. Alert titles use ` // ` as separator.
- **Explicit over clever.** `setting/env.go` maps every alert type to its env var with a switch, "deliberately not reflection-based, so every mapping is greppable".
- **Minimal dependencies.** Standard library first; no ORM-level abstractions beyond GORM, no DI container, no frontend component library.
- **Go formatting** is inconsistent in a few older files (config structs, some domain models are not gofmt-clean). Do not reformat unrelated files in a feature change.

## Recipes

### Add an alert type

1. `domain/notification/alert_types.go`: add the constant, a registry entry (category + default severity), and, if it is a fault with a duration, an `IncidentBindings` entry for both the failure and its recovery under one `IncidentKind`.
2. `config/alerts.go`: add `Severity…` field with its `ALERTSEVERITY_…` tag.
3. `application/setting/env.go`: add the case.
4. `.env.example`, `helm/values.yaml` (`config.alerts.severity`), `helm/templates/deployment.yaml` (env var).
5. Emit it with `notificationService.EmitAlert(...)` from a goroutine if you are on a hot path. Pass a `subject` if more than one of these can be open per machine.
6. Check the Settings page lists it (it reads the registry through `setting/view.go`).

### Add a WebSocket message

1. Add the type constant and payload struct in `domain/protocol/message.go`. New fields `omitempty`; never repurpose a field.
2. Agent: handle it in `readLoop` (`internal/agent/connection.go`), do the work in a goroutine, write the result through `wc.writeJSON`.
3. Control plane: send via `hub.Get().Send`, and for request/response use `hub.RegisterCommand` + a `*_result` case in `ws.go` that calls `hub.ResolveCommand`.
4. Remember older agents in the fleet will ignore the new type. Guard features on agent version if needed, and release the agent first.

### Add an endpoint

Route in `router.go` behind `middleware.AuthMiddleware()`, handler in `handlers/`, `EnforceRoles` check at the top, `BelongsToProject` (or the equivalent ownership check) before touching a machine or application by id. Service function in `application/`. Frontend: function in the matching `services/*.service.ts`, then `useQuery`/`useMutation` in the page.

### Add a persisted model

Domain model with `gorm.Model` in `domain/<name>/model.go`, repository interface next to it, two implementations (postgres and sqlite), factory function in `infrastructure/database/factory.go`, and the model added to both `Migrate()` lists.

### Add a screen

Copy the closest existing page (Hosts or Applications for list + detail). Keep the table grid, status cells, row menu, empty and loading states; on detail pages keep back link, header with icon/name/status, actions top-right, tabs, stat cards and 30-minute charts. Sort problems first. If the screen is workload-specific, gate its nav entry with a `role` in `Sidebar.tsx`.

## Tests

```
go test ./...           # 46 tests, all pass, SQLite in-memory where a DB is needed
go vet ./...            # clean
cd web && npx tsc -b    # clean; no test runner configured
```

Covered: incident open/close semantics, setting resolution order, machine reassignment (service and repository), SQL scope/edge/planned-window logic, SQL metrics folding, agent connection string building, project default protection. Not covered: hub concurrency, auth flows, heartbeat reconciliation of pools and sites, handlers, the whole frontend.

## Local development

- Control plane: copy `.env.example` to `.env` (defaults to SQLite at `./data/app.db` and memory cache), set at least `APPENV`, `APPURL`, `APPPORT`, `LOKIHOST`, `LOKIPORT`, `BOOTSTRAPTOKEN`, `AGENTVERSION`, `JWTSECRET`, `DEFAULTADMINUSERNAME`, `DEFAULTADMINPASSWORD`. Run `go run ./cmd/app` from the repo root (it reads `./data/*`).
- Web: `cd web && npm ci && npm run dev` with `VITE_API_BASE_URL` (and optionally `VITE_PROMETHEUS_URL`) in `web/.env`. CORS allows `http://localhost:5173`.
- Agent: needs a Windows host. With `APPENV=development` the generated bootstrap command adds an `ngrok-skip-browser-warning` header, which tells you how the owner develops: control plane on a laptop behind ngrok, a Windows VM running the bootstrap.
- `go run ./cmd/agent -v` prints the version.

## Build, version and release

Two independent version lines:

| Line | Where it is written | Bump with |
|---|---|---|
| Control plane + web (1.1.1) | `build-control-plane.yml`, `build-web.yaml` image tags, `helm/values.yaml` image tags, `Chart.yaml appVersion`, `web/package.json`, `handlers/health.go`, `.env.example` | `scripts/bump-app.sh [patch\|minor\|major\|x.y.z]` |
| Agent (1.0.4) | `build-agent.yml VERSION`, `helm/values.yaml agentVersion`, `.env.example AGENTVERSION` | `scripts/bump-agent.sh …` |

Both scripts also bump the chart patch version, and both dry-run every substitution before writing any file, failing loudly if a pattern no longer matches.

The image and agent workflows trigger only on changes to their own workflow file. Bumping the version edits that file, so pushing a bump to `main` is the release. The chart workflow triggers on any change under `helm/` and pushes to `oci://ghcr.io/su3i/charts`. Outputs:

| Artifact | Destination |
|---|---|
| `agent.exe` (windows/amd64, CGO off, stripped) | GitHub release `v<agent version>` on `su3i/wimp` |
| Control plane image | Docker Hub `darksueii/wimp:<version>` |
| Web image | Docker Hub `darksueii/wimp-web:<version>` |
| Helm chart | `ghcr.io/su3i/charts/wimp` |

Order matters when releasing an agent: the GitHub release must exist before any control plane tells hosts to download it, otherwise updates fail with a 404 and bootstrap breaks.

Commit messages follow `feat(<version>): …`, `fix(<version>): …`, `chore(): …`. There are no tags besides the agent release tags, and no CI test step.
