import { useState, useEffect, useMemo, lazy, Suspense } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { AlertTriangle, Monitor, Layers, X, ChevronRight, Info, CheckCircle2 } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useAuthStore } from "@/store/auth";
import { useProjectStore } from "@/store/project";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";
import { categoryIcon, categoryLabel, levelConfig } from "@/utils/notifications";
import { timeAgo } from "@/utils/time";
import { machineService } from "@/services/machine.service";
import { applicationService } from "@/services/application.service";
import { prometheusService, type PromInstantResult } from "@/services/prometheus.service";
import { dashboardService, type ActiveAlert, type DashboardNotification } from "@/services/dashboard.service";
import type { Application } from "@/types";

// Split out from pages/Dashboard.tsx into their own chunk since recharts is a large
// dependency - the rest of the dashboard (header, stat cards) doesn't need to wait on it.
const RadialGauge = lazy(() =>
  import("@/components/dashboard/DashboardCharts").then((m) => ({ default: m.RadialGauge })),
);
const HostCpuLineChart = lazy(() =>
  import("@/components/dashboard/DashboardCharts").then((m) => ({ default: m.HostCpuLineChart })),
);

function mids(ids: number[]) {
  return `machine_id=~"${ids.join("|")}"`;
}

const PQ = {
  // Total IIS request rate across all hosts and sites (req/s)
  throughput: (ids: number[]) => `sum(rate(windows_iis_requests_total{${mids(ids)}}[5m]))`,

  // Average CPU % across all project hosts
  cpuAvg: (ids: number[]) =>
    `avg(100 - (avg by (machine_id) (rate(windows_cpu_time_total{${mids(ids)},mode="idle"}[5m])) * 100))`,

  // Average memory used % across all project hosts
  memAvg: (ids: number[]) =>
    `avg(100 - (windows_memory_physical_free_bytes{${mids(ids)}} / windows_memory_physical_total_bytes{${mids(
      ids,
    )}} * 100))`,

  networkIn: (ids: number[]) =>
    `sum(rate(windows_net_bytes_received_total{${mids(ids)},nic!~".*isatap.*"}[5m]))`,

  networkOut: (ids: number[]) =>
    `sum(rate(windows_net_bytes_sent_total{${mids(ids)},nic!~".*isatap.*"}[5m]))`,

  // Per-host CPU %, one result per machine_id label - used for comparison chart
  cpuPerHost: (ids: number[]) =>
    `100 - (avg by (machine_id) (rate(windows_cpu_time_total{${mids(ids)},mode="idle"}[5m])) * 100)`,
};

// Extracts a single scalar from a Prometheus instant result (handles NaN/Inf)
function scalar(r: PromInstantResult[] | undefined): number | null {
  const v = r?.[0]?.value?.[1];
  if (v == null) return null;
  const n = parseFloat(v);
  return isFinite(n) ? n : null;
}

function fmtRate(v: number) {
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k/s`;
  return `${v.toFixed(1)}/s`;
}

function fmtBytes(v: number) {
  if (v >= 1_073_741_824) return `${(v / 1_073_741_824).toFixed(1)} GB/s`;
  if (v >= 1_048_576) return `${(v / 1_048_576).toFixed(1)} MB/s`;
  if (v >= 1_024) return `${(v / 1_024).toFixed(1)} KB/s`;
  return `${Math.round(v)} B/s`;
}

function alertIcon(cat: string | null | undefined): LucideIcon {
  if (cat === "machine") return Monitor;
  if (cat === "apppool" || cat === "app_pool") return Layers;
  return AlertTriangle;
}

function AlertRow({ alert, onDismiss }: { alert: ActiveAlert; onDismiss: () => void }) {
  const Icon = alertIcon(alert.category);
  return (
    <div className='flex items-center gap-3 px-4 py-2.5 bg-danger text-white border-b border-red-700/40 last:border-0'>
      <span className='shrink-0 text-base'>🚨</span>
      <Icon className='size-4 shrink-0 opacity-75' />
      <span className='flex-1 text-sm'>{alert.message ?? "System event"}</span>
      <span className='text-xs opacity-60 shrink-0 whitespace-nowrap'>{timeAgo(alert.fired_at)}</span>
      <button
        type='button'
        onClick={onDismiss}
        aria-label='Dismiss'
        className='shrink-0 rounded p-0.5 hover:bg-surface-high transition-colors cursor-pointer'
      >
        <X className='size-3.5' />
      </button>
    </div>
  );
}

function InfoTooltip({ text }: { text: string }) {
  return (
    <span className='relative inline-flex group/info'>
      <Info className='size-3 text-ink-faint hover:text-ink-dim transition-colors cursor-help' />
      <span className='pointer-events-none absolute top-full left-1/2 -translate-x-1/2 mt-1.5 w-max max-w-[190px] px-2 py-1.5 rounded border border-rim bg-surface-highest text-[0.5625rem] normal-case tracking-normal font-normal text-ink-dim leading-snug opacity-0 group-hover/info:opacity-100 transition-opacity z-20 shadow-md'>
        {text}
      </span>
    </span>
  );
}

function MetricCard({
  label,
  value,
  sub,
  info,
}: {
  label: string;
  value: string | null;
  sub?: string;
  info?: string;
}) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-[18px] py-[18px] flex flex-col gap-3'>
      <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        {label}
        {info && <InfoTooltip text={info} />}
      </span>
      <span className='text-[27px] font-semibold font-mono leading-none text-ink'>
        {value ?? "N/A"}
      </span>
      {sub && <span className='text-[0.625rem] text-ink-faint'>{sub}</span>}
    </div>
  );
}

function SevEventsCard({ count }: { count: number }) {
  const hot = count > 0;
  return (
    <div className={cn(
      'rounded-lg border bg-surface px-[18px] py-[18px] flex flex-col gap-3 transition-colors border-rim',
    )}>
      <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        Sev+ Events
        <InfoTooltip text='Distinct Sev incidents in the last 24h.' />
      </span>
      <span className={cn('text-[27px] font-semibold font-mono leading-none', hot ? 'text-danger' : 'text-ink')}>
        {count}
      </span>
      <span className='text-[0.625rem] text-ink-faint'>last 24h</span>
    </div>
  );
}

// Up/Down is only ever reported for apps with a health check URL configured - that's
// the one signal that actually means "is this app reachable". App pool state says
// nothing about that (a pool can be Started with a dead site behind it), so apps
// without a health check show their live pool count instead of a fabricated status.
type AppRowStatus =
  | { kind: "healthcheck"; label: "Up" | "Down" | "No Data"; dot: string; text: string; uptimePct: string | null }
  | { kind: "pools"; healthy: number; total: number };

function appRowStatus(
  app: Application,
  hcStatusMap: Record<number, "up" | "down">,
  hcUptimeMap: Record<number, number>,
): AppRowStatus {
  if (app.HealthCheckURL) {
    const raw = hcStatusMap[app.ID];
    const label = raw === "up" ? "Up" : raw === "down" ? "Down" : "No Data";
    const dot = label === "Up" ? "bg-success" : label === "Down" ? "bg-danger animate-pulse" : "bg-ink-faint/40";
    const text = label === "Up" ? "text-success" : label === "Down" ? "text-danger" : "text-ink-faint";
    const ratio = hcUptimeMap[app.ID];
    // Exact value, not rounded in either direction - .toFixed(2) is display precision,
    // not rounding-to-mislead (matches ApplicationDetail.tsx's HealthMonitor exactly).
    const uptimePct = ratio != null ? `${(ratio * 100).toFixed(2)}%` : null;
    return { kind: "healthcheck", label, dot, text, uptimePct };
  }
  return { kind: "pools", healthy: app.pool_healthy ?? 0, total: app.pool_total ?? 0 };
}

function ApplicationStatusCard({
  apps,
  hcStatusMap,
  hcUptimeMap,
}: {
  apps: Application[];
  hcStatusMap: Record<number, "up" | "down">;
  hcUptimeMap: Record<number, number>;
}) {
  const navigate = useNavigate();

  return (
    <div className='min-w-0 max-h-[140px] rounded-lg border border-rim bg-surface px-[18px] py-[14px] flex flex-col gap-2 overflow-hidden'>
      <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none shrink-0'>
        Applications
        <InfoTooltip text='Up/Down reflects the health check URL when one is configured; otherwise the live app pool count is shown.' />
      </span>

      {/* flex-1 always: this fills exactly whatever vertical space is left in the card
          (which itself is height-locked by its sibling cards via the row-1 grid, not by
          its own content) - so the list can never push the card taller, no matter how
          many applications there are. 0-1 apps center in that space; more than that
          scrolls internally instead of growing. */}
      <div
        className={cn(
          'min-w-0 min-h-0 flex-1 flex flex-col overflow-x-hidden',
          apps.length <= 1 ? 'justify-center' : 'overflow-y-auto',
        )}
      >
        {apps.length === 0 ? (
          <p className='text-center text-xs text-ink-faint'>No applications found.</p>
        ) : (
          apps.map((app) => {
            const status = appRowStatus(app, hcStatusMap, hcUptimeMap);
            return (
              <div key={app.ID} className='grid grid-cols-[1fr_auto_auto] items-center gap-3 py-1.5'>
                <button
                  type='button'
                  onClick={() => navigate(`/applications/${app.ID}`)}
                  className='min-w-0 truncate rounded text-left text-xs text-ink underline-offset-2 underline cursor-pointer'
                >
                  {app.Name}
                </button>
                {status.kind === "healthcheck" ? (
                  <span className={cn('flex items-center gap-1.5 text-[0.6875rem] font-medium', status.text)}>
                    {status.label === "Up"
                      ? <CheckCircle2 className='size-3 shrink-0' />
                      : <span className={cn('size-2 rounded-full shrink-0', status.dot)} />}
                    {status.label}
                  </span>
                ) : (
                  <span className='font-mono text-[0.6875rem] text-ink-dim'>
                    {status.healthy}/{status.total}
                  </span>
                )}
                {status.kind === "healthcheck" && status.uptimePct ? (
                  <span className='flex items-baseline gap-1 whitespace-nowrap text-ink'>
                    <span className='text-[0.6875rem]'>{status.uptimePct}</span>
                    <span className='text-[0.5625rem]'>uptime</span>
                  </span>
                ) : (
                  <span />
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

function BandwidthCard({ inVal, outVal }: { inVal: number | null; outVal: number | null }) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-[18px] py-[18px] flex flex-col gap-3'>
      <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        Bandwidth
        <InfoTooltip text='Total network in/out across all hosts, 5-min average.' />
      </span>
      <div className='flex items-end gap-4'>
        <span className='text-[22px] font-semibold font-mono leading-none text-ink'>
          {inVal != null ? fmtBytes(inVal) : 'N/A'}
          <span className='ml-1.5 text-[0.625rem] text-ink-faint'>IN</span>
        </span>
        <span className='text-ink-faint/40 text-base font-light mb-0.5'>/</span>
        <span className='text-[22px] font-semibold font-mono leading-none text-ink'>
          {outVal != null ? fmtBytes(outVal) : 'N/A'}
          <span className='ml-1.5 text-[0.625rem] text-ink-faint'>OUT</span>
        </span>
      </div>
      <span className='text-[0.625rem] text-ink-faint'>last 5 min</span>
    </div>
  );
}

function LevelBadge({ level }: { level: string }) {
  const cfg = levelConfig(level);
  return (
    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-[0.625rem] font-semibold uppercase tracking-wide whitespace-nowrap", cfg.cls)}>
      {cfg.label}
    </span>
  );
}

function NotifRow({ notif }: { notif: DashboardNotification }) {
  const CatIcon = categoryIcon(notif.Category);
  return (
    <div className='grid grid-cols-[84px_1fr_124px_92px] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'>
      <div className='px-4 py-[9px]'>
        <LevelBadge level={notif.Level ?? "info"} />
      </div>
      <div className='px-4 py-[9px] min-w-0 flex items-center gap-1.5 overflow-hidden'>
        <span className='text-xs font-medium text-ink shrink-0 truncate'>{notif.Title ?? ""}</span>
        {notif.Detail && (
          <>
            <span className='text-ink-faint/40 text-sm shrink-0'>/</span>
            <span className='text-xs text-ink-dim truncate'>{notif.Detail.toLowerCase()}</span>
          </>
        )}
      </div>
      <div className='flex items-center gap-1.5 px-4 py-[9px]'>
        <CatIcon className='size-3 text-ink-faint shrink-0' />
        <span className='text-xs text-ink-dim'>{categoryLabel(notif.Category)}</span>
      </div>
      <div className='px-4 py-[9px] text-right'>
        <span className='text-xs text-ink-faint tabular-nums'>{timeAgo(notif.CreatedAt)}</span>
      </div>
    </div>
  );
}

function NotifRowSkeleton() {
  return (
    <div className='grid grid-cols-[84px_1fr_124px_92px] items-center border-b border-rim last:border-0 animate-pulse'>
      <div className='px-4 py-[9px]'>
        <div className='h-4 w-12 rounded bg-surface-high' />
      </div>
      <div className='px-4 py-[9px]'>
        <div className='h-2.5 w-2/3 rounded bg-surface-high' />
      </div>
      <div className='flex items-center px-4 py-[9px]'>
        <div className='h-2.5 w-16 rounded bg-surface-high' />
      </div>
      <div className='px-4 py-[9px] flex justify-end'>
        <div className='h-2.5 w-12 rounded bg-surface-high' />
      </div>
    </div>
  );
}

function ChartSkeleton({ height }: { height: number }) {
  return (
    <div className='flex items-center justify-center animate-pulse' style={{ height }}>
      <div className='h-full w-full rounded bg-surface-high' />
    </div>
  );
}

export function Dashboard() {
  usePageTitle("Overview");
  const queryClient = useQueryClient();
  const { activeProject } = useProjectStore();
  const projectKey = activeProject?.Key ?? "";

  const [alerts, setAlerts] = useState<ActiveAlert[]>([]);
  const [dismissed, setDismissed] = useState<Set<string>>(new Set());

  const { data: machines } = useQuery({
    queryKey: ["machines", projectKey],
    queryFn: async () => {
      const { data } = await machineService.list(projectKey, { per_page: 100 });
      return data.machines ?? [];
    },
    enabled: !!projectKey,
    staleTime: 60_000,
  });

  const machineIds = useMemo(() => (machines ?? []).map((m) => m.ID), [machines]);

  const hostNameMap = useMemo(() => {
    const map = new Map<number, string>();
    for (const m of machines ?? []) {
      map.set(m.ID, m.Hostname?.toLowerCase() ?? String(m.ID));
    }
    return map;
  }, [machines]);

  // Stable string key for queryKey (avoids array reference churn)
  const idKey = machineIds
    .slice()
    .sort((a, b) => a - b)
    .join(",");

  const promOk = prometheusService.isConfigured();
  const promEnabled = promOk && machineIds.length > 0;

  const qOpts = { refetchInterval: 5_000, staleTime: 0, enabled: promEnabled } as const;

  const { data: rThroughput } = useQuery({
    queryKey: ["d-throughput", idKey],
    queryFn: () => prometheusService.instant(PQ.throughput(machineIds)),
    ...qOpts,
  });
  const { data: rCpuAvg } = useQuery({
    queryKey: ["d-cpuavg", idKey],
    queryFn: () => prometheusService.instant(PQ.cpuAvg(machineIds)),
    ...qOpts,
  });
  const { data: rMemAvg } = useQuery({
    queryKey: ["d-memavg", idKey],
    queryFn: () => prometheusService.instant(PQ.memAvg(machineIds)),
    ...qOpts,
  });
  const { data: rNetIn } = useQuery({
    queryKey: ["d-net-in", idKey],
    queryFn: () => prometheusService.instant(PQ.networkIn(machineIds)),
    ...qOpts,
  });
  const { data: rNetOut } = useQuery({
    queryKey: ["d-net-out", idKey],
    queryFn: () => prometheusService.instant(PQ.networkOut(machineIds)),
    ...qOpts,
  });
  const { data: rCpuHost } = useQuery({
    queryKey: ["d-cpuhost", idKey],
    queryFn: () => {
      const now = Math.floor(Date.now() / 1000);
      return prometheusService.range(PQ.cpuPerHost(machineIds), now - 60 * 60, now, 60);
    },
    ...qOpts,
  });

  const { data: stats } = useQuery({
    queryKey: ["dashboard-stats", projectKey],
    queryFn: () => dashboardService.getStats(projectKey!),
    enabled: !!projectKey,
    refetchInterval: 30_000,
    staleTime: 0,
  });

  const { data: dashboardApps = [] } = useQuery({
    queryKey: ["dashboard-applications", projectKey],
    queryFn: async () => {
      const { data } = await applicationService.list(projectKey!, { page: 1, per_page: 100 });
      return data.applications;
    },
    enabled: !!projectKey,
    refetchInterval: 30_000,
  });

  // Only applications with a health check URL configured have a real up/down signal -
  // same blackbox_http probe_success metric ApplicationDetail.tsx's HealthMonitor reads
  // for a single app, batched here across every app_id at once via a regex label match
  // (same batching trick as PQ.cpuPerHost for machine_id).
  const hcAppIds = dashboardApps.filter((a) => !!a.HealthCheckURL).map((a) => a.ID);
  const hcIdKey = hcAppIds.slice().sort((a, b) => a - b).join(",");
  const hcEnabled = promOk && hcAppIds.length > 0;

  const { data: hcStatusResult } = useQuery({
    queryKey: ["d-app-hc-status", hcIdKey],
    queryFn: () => prometheusService.instant(`probe_success{job="blackbox_http", application_id=~"${hcIdKey.split(",").join("|")}"}`),
    enabled: hcEnabled,
    refetchInterval: 30_000,
  });
  const { data: hcUptimeResult } = useQuery({
    queryKey: ["d-app-hc-uptime", hcIdKey],
    queryFn: () => prometheusService.instant(`avg_over_time(probe_success{job="blackbox_http", application_id=~"${hcIdKey.split(",").join("|")}"}[30d])`),
    enabled: hcEnabled,
    refetchInterval: 60_000,
  });

  const hcStatusMap = useMemo(() => {
    const m: Record<number, "up" | "down"> = {};
    for (const r of hcStatusResult ?? []) {
      const id = Number(r.metric.application_id);
      if (!Number.isNaN(id)) m[id] = r.value[1] === "1" ? "up" : "down";
    }
    return m;
  }, [hcStatusResult]);

  const hcUptimeMap = useMemo(() => {
    const m: Record<number, number> = {};
    for (const r of hcUptimeResult ?? []) {
      const id = Number(r.metric.application_id);
      if (!Number.isNaN(id)) m[id] = Number(r.value[1]);
    }
    return m;
  }, [hcUptimeResult]);

  const { data: notifications = [], isLoading: notificationsLoading } = useQuery({
    queryKey: ["dashboard-notifications", projectKey],
    queryFn: () => dashboardService.getNotifications(projectKey || undefined),
    enabled: !!projectKey,
    refetchInterval: 5_000,
    select: (d) => d ?? [],
  });

  useEffect(() => {
    void dashboardService.getActiveAlerts().then((d) => setAlerts(d ?? []));
  }, []);

  useEffect(() => {
    const { accessToken: rawAccessToken } = useAuthStore.getState();
    if (!rawAccessToken) return;
    const accessToken: string = rawAccessToken;

    const base = (import.meta.env.VITE_API_BASE_URL as string)
      .replace(/\/api\/v1.*$/, "")
      .replace(/^http/, "ws");

    const MIN_RECONNECT_DELAY_MS = 1_000;
    const MAX_RECONNECT_DELAY_MS = 30_000;

    let cancelled = false;
    let ws: WebSocket | null = null;
    let reconnectDelay = MIN_RECONNECT_DELAY_MS;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    function scheduleReconnect() {
      if (cancelled) return;
      reconnectTimer = setTimeout(() => {
        reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY_MS);
        connect();
      }, reconnectDelay);
    }

    function connect() {
      if (cancelled) return;
      try {
        ws = new WebSocket(`${base}/ws?token=${encodeURIComponent(accessToken)}`);
      } catch {
        scheduleReconnect();
        return;
      }

      ws.onopen = () => {
        reconnectDelay = MIN_RECONNECT_DELAY_MS;
      };

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data as string) as { type: string; payload: unknown };
          switch (msg.type) {
            case "alert_fired": {
              const a = msg.payload as ActiveAlert;
              setAlerts((prev) => (prev.some((x) => x.id === a.id) ? prev : [...prev, a]));
              break;
            }
            case "alert_resolved": {
              const { alert_id } = msg.payload as { alert_id: string };
              setAlerts((prev) => prev.filter((x) => x.id !== alert_id));
              break;
            }
            case "notification": {
              queryClient.invalidateQueries({ queryKey: ["dashboard-notifications", projectKey] });
              break;
            }
          }
        } catch {
          /* ignore malformed frames */
        }
      };

      // onerror is always followed by onclose (per the WebSocket spec), which is
      // where reconnection is scheduled - nothing extra to do here beyond not crashing.
      ws.onerror = () => {
        ws?.close();
      };

      ws.onclose = () => {
        scheduleReconnect();
      };
    }

    connect();

    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      ws?.close();
    };
  }, [queryClient, projectKey]);

  const sevLast24h = stats?.sev_last_24h ?? 0;
  const throughputVal = scalar(rThroughput);
  const cpuAvgVal = scalar(rCpuAvg);
  const memAvgVal = scalar(rMemAvg);
  const netInVal = scalar(rNetIn);
  const netOutVal = scalar(rNetOut);

  // Per-host CPU time-series, one row per timestamp, one key per machine
  const hostPerfData = useMemo(() => {
    if (!rCpuHost?.length) return { rows: [] as Record<string, string | number>[], keys: [] as string[] };
    const keys = rCpuHost.map((s) => hostNameMap.get(Number(s.metric.machine_id)) ?? s.metric.machine_id ?? "?");
    const timestamps = rCpuHost[0]?.values?.map(([ts]) => ts) ?? [];
    const rows = timestamps.map((ts, i) => {
      const row: Record<string, string | number> = {
        time: new Date(ts * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
      };
      rCpuHost.forEach((s, si) => {
        const val = parseFloat(s.values[i]?.[1] ?? "0");
        row[keys[si]] = isFinite(val) ? parseFloat(val.toFixed(1)) : 0;
      });
      return row;
    });
    return { rows, keys };
  }, [rCpuHost, hostNameMap]);

  const visibleAlerts = alerts.filter((a) => a.id && !dismissed.has(a.id));
  function dismiss(id: string) {
    setDismissed((prev) => new Set([...prev, id]));
  }

  return (
    <>
      {/* Alert Banner */}
      {visibleAlerts.length > 0 && (
        <div className='-mt-6 -mx-6 mb-4 sticky top-0 z-50 shadow-md'>
          {visibleAlerts.map((a) => (
            <AlertRow key={a.id} alert={a} onDismiss={() => dismiss(a.id)} />
          ))}
        </div>
      )}

      {/* Header */}
      <div className='mb-[22px]'>
        <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mb-2'>Overview</p>
        <h1 className='text-[22px] font-semibold text-ink tracking-tight'>{activeProject?.Name ?? "Overview"}</h1>
      </div>

      <div className='space-y-[14px]'>
        {/* ── Row 1: 4 aggregate stat cards ───────────────────────────── */}
        <div className='grid grid-cols-[0.85fr_1.15fr_1fr_1.5fr] gap-[14px]'>
          <SevEventsCard count={sevLast24h} />
          <ApplicationStatusCard apps={dashboardApps} hcStatusMap={hcStatusMap} hcUptimeMap={hcUptimeMap} />
          <MetricCard
            label='Request Throughput'
            value={throughputVal != null ? fmtRate(throughputVal) : null}
            sub='last 5 min'
            info='Total IIS request rate across this project, 5-min average.'
          />
          <BandwidthCard
            inVal={netInVal}
            outVal={netOutVal}
          />
        </div>

        {/* ── Row 2: Capacity Overview + Host Performance ──────────────── */}
        <div className='grid grid-cols-5 gap-[14px]'>
          {/* Capacity Overview */}
          <div className='col-span-2 rounded-lg border border-rim bg-surface p-4 flex flex-col gap-[14px]'>
            <p className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Capacity Overview
              <InfoTooltip text='Live average CPU and memory across online hosts.' />
            </p>
            <div className='grid grid-cols-2 gap-2 flex-1'>
              <Suspense fallback={<ChartSkeleton height={162} />}>
                <RadialGauge label='CPU Avg' value={cpuAvgVal} />
              </Suspense>
              <Suspense fallback={<ChartSkeleton height={162} />}>
                <RadialGauge label='Memory Avg' value={memAvgVal} />
              </Suspense>
            </div>
          </div>

          {/* Host Performance, per-host CPU time series */}
          <div className='col-span-3 rounded-lg border border-rim bg-surface p-4 flex flex-col gap-[11px]'>
            <p className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Host CPU %
              <InfoTooltip text='Live per-host CPU over time.' />
            </p>
            <Suspense fallback={<ChartSkeleton height={234} />}>
              <HostCpuLineChart rows={hostPerfData.rows} keys={hostPerfData.keys} />
            </Suspense>
          </div>
        </div>

        {/* ── Row 3: Recent Alerts ─────────────────────────────────────── */}
        <div className='rounded-lg border border-rim overflow-hidden'>
          <div className='flex items-center justify-between px-4 py-3 border-b border-rim bg-surface-alt'>
            <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Recent Activity
            </p>
            <Link
              to='/activity'
              className='flex items-center gap-1 text-xs text-ink-faint hover:text-ink transition-colors'
            >
              View All <ChevronRight className='size-3.5' />
            </Link>
          </div>
          {notificationsLoading ? (
            <div className='bg-surface'>
              {Array.from({ length: 4 }).map((_, i) => (
                <NotifRowSkeleton key={i} />
              ))}
            </div>
          ) : notifications.length > 0 ? (
            <div className='max-h-[259px] overflow-y-auto bg-surface'>
              {notifications.map((n, i) => (
                <NotifRow key={n.ID ?? i} notif={n} />
              ))}
            </div>
          ) : (
            <div className='flex h-24 items-center justify-center text-xs text-ink-faint bg-surface'>
              No recent activity
            </div>
          )}
        </div>
      </div>
    </>
  );
}
