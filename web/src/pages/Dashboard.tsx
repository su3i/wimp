import { useState, useEffect, useMemo, lazy, Suspense } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { AlertTriangle, Monitor, Layers, X, ChevronRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { InfoTooltip } from "@/components/ui/InfoTooltip";
import { DiskGauges } from "@/components/dashboard/DiskGauges";
import { useAuthStore } from "@/store/auth";
import { useProjectStore } from "@/store/project";
import { useUIStore } from "@/store/ui";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";
import { levelConfig, splitTitle } from "@/utils/notifications";
import { timeAgo, absoluteTime } from "@/utils/time";
import { machineService } from "@/services/machine.service";
import { applicationService } from "@/services/application.service";
import { prometheusService, type PromInstantResult, type PromRangeResult } from "@/services/prometheus.service";
import { dashboardService, type ActiveAlert, type DashboardNotification } from "@/services/dashboard.service";
import type { Application } from "@/types";

// Split out from pages/Dashboard.tsx into their own chunk since recharts is a large
// dependency - the rest of the dashboard (header, stat cards) doesn't need to wait on it.
const HostLineChart = lazy(() =>
  import("@/components/dashboard/DashboardCharts").then((m) => ({ default: m.HostLineChart })),
);

function mids(ids: number[]) {
  return `machine_id=~"${ids.join("|")}"`;
}

const PQ = {
  networkIn: (ids: number[]) =>
    `sum(rate(windows_net_bytes_received_total{${mids(ids)},nic!~".*isatap.*"}[5m]))`,

  networkOut: (ids: number[]) =>
    `sum(rate(windows_net_bytes_sent_total{${mids(ids)},nic!~".*isatap.*"}[5m]))`,

  // Per-host IIS request rate, one result per machine_id label - used for the Request
  // Throughput chart (the same metric as `throughput` above, just not summed down to one
  // series so each host is its own line).
  throughputPerHost: (ids: number[], win: string) =>
    `sum by (machine_id) (rate(windows_iis_requests_total{${mids(ids)}}[${win}]))`,

  // Per-host CPU %, one result per machine_id label - used for the Cpu Usage chart
  cpuPerHost: (ids: number[], win: string) =>
    `100 - (avg by (machine_id) (rate(windows_cpu_time_total{${mids(ids)},mode="idle"}[${win}])) * 100)`,

  // Per-host memory used %, one result per machine_id label - used for the Memory Usage chart
  memPerHost: (ids: number[]) =>
    `100 - (windows_memory_physical_free_bytes{${mids(ids)}} / windows_memory_physical_total_bytes{${mids(ids)}} * 100)`,
};

// Selectable windows for the three per-host line charts. `step` keeps every window at
// roughly 60 points (so a 3-day chart isn't 4320 samples wide), and `rateWindow` scales
// with it so each plotted point still averages over the gap it represents rather than a
// fixed 5 minutes of a 15-minute step. `refetchMs` scales too: a 3-day chart moves by one
// pixel every 15 minutes, so re-querying it every 5s would just hammer Prometheus with a
// far more expensive query for no visible change.
type RangeKey = "1h" | "24h" | "3d";

const RANGES: Record<
  RangeKey,
  { label: string; seconds: number; step: number; rateWindow: string; refetchMs: number }
> = {
  "1h": { label: "1h", seconds: 60 * 60, step: 60, rateWindow: "5m", refetchMs: 5_000 },
  "24h": { label: "24h", seconds: 24 * 60 * 60, step: 300, rateWindow: "10m", refetchMs: 30_000 },
  "3d": { label: "3d", seconds: 3 * 24 * 60 * 60, step: 900, rateWindow: "30m", refetchMs: 60_000 },
};

const RANGE_KEYS: RangeKey[] = ["1h", "24h", "3d"];

// Spelled out in the dropdown (unlike the compact axis-style "1h"/"24h"/"3d" keys) since
// a closed select shows only the current choice, with no siblings to give it context.
const RANGE_OPTION_LABELS: Record<RangeKey, string> = {
  "1h": "Last hour",
  "24h": "Last 24 hours",
  "3d": "Last 3 days",
};

// Resolves a range key into the concrete arguments a Prometheus range query needs. Lives
// at module scope and is called from inside the query function, never during render: the
// window has to be anchored to the moment the request actually goes out, or a component
// that re-renders without refetching would silently shift the window under a cached result.
function rangeWindow(range: RangeKey) {
  const { seconds, step, rateWindow } = RANGES[range];
  const end = Math.floor(Date.now() / 1000);
  return { start: end - seconds, end, step, rateWindow };
}

// A bare clock reads as ambiguous the moment the window spans more than one day, so
// anything past 24h gets a weekday prefix.
function fmtChartTime(ts: number, range: RangeKey) {
  const d = new Date(ts * 1000);
  const clock = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (range === "3d") return `${d.toLocaleDateString([], { weekday: "short" })} ${clock}`;
  return clock;
}

function RangeToggle({ value, onChange }: { value: RangeKey; onChange: (v: RangeKey) => void }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as RangeKey)}
      aria-label='Chart time range'
      className='cursor-pointer rounded-md border border-rim bg-surface-alt px-2 py-1 text-[0.625rem] font-medium tracking-normal normal-case text-ink-dim hover:text-ink focus:outline-none focus:border-primary transition-colors'
    >
      {RANGE_KEYS.map((key) => (
        <option key={key} value={key}>
          {RANGE_OPTION_LABELS[key]}
        </option>
      ))}
    </select>
  );
}

// Extracts a single scalar from a Prometheus instant result (handles NaN/Inf)
function scalar(r: PromInstantResult[] | undefined): number | null {
  const v = r?.[0]?.value?.[1];
  if (v == null) return null;
  const n = parseFloat(v);
  return isFinite(n) ? n : null;
}

// Reshapes a per-host Prometheus range result (one series per machine_id) into
// one row per timestamp with one column per hostname - the shape HostLineChart needs.
// Shared by the Request Throughput, Cpu Usage and Memory Usage charts.
function toHostRows(
  range: PromRangeResult[] | undefined,
  hostNameMap: Map<number, string>,
  rangeKey: RangeKey,
) {
  if (!range?.length) return { rows: [] as Record<string, string | number>[], keys: [] as string[] };
  const keys = range.map((s) => hostNameMap.get(Number(s.metric.machine_id)) ?? s.metric.machine_id ?? "?");
  const timestamps = range[0]?.values?.map(([ts]) => ts) ?? [];
  const rows = timestamps.map((ts, i) => {
    const row: Record<string, string | number> = { time: fmtChartTime(ts, rangeKey) };
    range.forEach((s, si) => {
      const val = parseFloat(s.values[i]?.[1] ?? "0");
      row[keys[si]] = isFinite(val) ? parseFloat(val.toFixed(1)) : 0;
    });
    return row;
  });
  return { rows, keys };
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

// Shared shell for the aggregate tiles in the top row. justify-between rather than a
// fixed gap: the row's height is set by the tallest card in it (the disk gauges), and
// spreading label to top / value to bottom keeps the shorter tiles from stacking their
// content up top with dead space underneath.
const STAT_CARD =
  "rounded-lg border border-rim bg-surface px-[18px] py-[18px] flex flex-col justify-between gap-3 min-h-[124px]";

function SevEventsCard({ count }: { count: number }) {
  const hot = count > 0;
  return (
    <div className={cn(STAT_CARD, 'transition-colors')}>
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

// A "healthy" app is Up on its health check when one is configured, otherwise fully
// healthy on live app pool state - the same per-app source of truth used on the
// Applications list page, just rolled up into one count here.
function isAppHealthy(app: Application, hcStatusMap: Record<number, "up" | "down">): boolean {
  if (app.HealthCheckURL) return hcStatusMap[app.ID] === "up";
  const total = app.pool_total ?? 0;
  return total > 0 && app.pool_healthy === total;
}

function ApplicationStatusCard({
  apps,
  hcStatusMap,
  hcUptimeMap,
  ready,
}: {
  apps: Application[];
  hcStatusMap: Record<number, "up" | "down">;
  hcUptimeMap: Record<number, number>;
  // False while health-check status has been requested but hasn't arrived yet - without
  // this, apps that are actually healthy would briefly count as down (no data yet reads
  // the same as "down"), flashing a misleading 0/N before settling on the real count.
  ready: boolean;
}) {
  const total = apps.length;
  const healthy = apps.filter((a) => isAppHealthy(a, hcStatusMap)).length;
  const showCount = ready && total > 0;

  const uptimeRatios = Object.values(hcUptimeMap);
  const avgUptime =
    uptimeRatios.length > 0
      ? (uptimeRatios.reduce((sum, r) => sum + r, 0) / uptimeRatios.length) * 100
      : null;

  const color =
    !showCount ? "text-ink" : healthy === total ? "text-success" : healthy === 0 ? "text-danger" : "text-warning";

  return (
    <div className={STAT_CARD}>
      <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        Applications
        <InfoTooltip text='Counts an app healthy via its health check URL when configured, otherwise by live app pool state.' />
      </span>
      <span className={cn('flex items-baseline gap-1.5', color)}>
        {!showCount ? (
          <span className='text-[27px] font-semibold font-mono leading-none'>N/A</span>
        ) : (
          <>
            <span className='text-[27px] font-semibold font-mono leading-none'>{healthy}/{total}</span>
            <span className='text-xs font-medium'>Healthy</span>
          </>
        )}
      </span>
      <span className='text-[0.625rem] text-ink-faint'>
        {avgUptime != null ? `${avgUptime.toFixed(2)}% avg uptime` : "no data"}
      </span>
    </div>
  );
}

function BandwidthCard({ inVal, outVal }: { inVal: number | null; outVal: number | null }) {
  return (
    <div className={STAT_CARD}>
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
  const { host, event } = splitTitle(notif.Title);
  return (
    // No items-center at the row's grid level - see Applications.tsx/Alerts.tsx for why
    // (the Event cell is one or two lines depending on Detail; every cell stretches to
    // the full row height and centers its own content instead).
    <div className='grid grid-cols-[84px_120px_1fr_190px] border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'>
      <div className='flex items-center px-4 py-[9px]'>
        <LevelBadge level={notif.Level ?? "info"} />
      </div>
      <div className='flex min-w-0 items-center px-4 py-[9px]'>
        <span className='font-mono text-xs text-ink-dim truncate block'>{host.toLowerCase() || "—"}</span>
      </div>
      <div className='flex min-w-0 flex-col justify-center px-4 py-[9px]'>
        <p className='text-xs font-medium text-ink truncate'>{event}</p>
        {notif.Detail && (
          <p className='text-[0.6875rem] text-ink-faint truncate'>{notif.Detail.toLowerCase()}</p>
        )}
      </div>
      <div className='flex flex-col items-end justify-center px-4 py-[9px]'>
        <span className='text-xs text-ink-faint tabular-nums whitespace-nowrap'>{timeAgo(notif.CreatedAt)}</span>
        <span className='text-[0.625rem] text-ink-faint/60 tabular-nums whitespace-nowrap'>{absoluteTime(notif.CreatedAt)}</span>
      </div>
    </div>
  );
}

// Mirrors NotifRow's exact structure (same two-line Event column) so the loading state
// is the same height as a loaded row - swapping one for the other shouldn't shift layout.
function NotifRowSkeleton() {
  return (
    <div className='grid grid-cols-[84px_120px_1fr_190px] border-b border-rim last:border-0 animate-pulse'>
      <div className='flex items-center px-4 py-[9px]'>
        <div className='h-4 w-12 rounded bg-surface-high' />
      </div>
      <div className='flex items-center px-4 py-[9px]'>
        <div className='h-2.5 w-16 rounded bg-surface-high' />
      </div>
      <div className='flex flex-col justify-center gap-1.5 px-4 py-[9px]'>
        <div className='h-2.5 w-2/3 rounded bg-surface-high' />
        <div className='h-2 w-1/3 rounded bg-surface-high' />
      </div>
      <div className='flex flex-col items-end justify-center gap-1 px-4 py-[9px]'>
        <div className='h-2.5 w-14 rounded bg-surface-high' />
        <div className='h-2 w-20 rounded bg-surface-high' />
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

// Shared shell for the three per-host line charts - title, info tooltip, window toggle,
// and the Suspense boundary the lazily-loaded recharts chunk lands in.
function ChartCard({
  title,
  info,
  range,
  onRangeChange,
  children,
}: {
  title: string;
  info: string;
  range: RangeKey;
  onRangeChange: (v: RangeKey) => void;
  children: React.ReactNode;
}) {
  return (
    <div className='rounded-lg border border-rim bg-surface p-4 flex flex-col gap-[11px]'>
      <div className='flex items-center justify-between gap-2'>
        <p className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
          {title}
          <InfoTooltip text={info} />
        </p>
        <RangeToggle value={range} onChange={onRangeChange} />
      </div>
      <Suspense fallback={<ChartSkeleton height={234} />}>{children}</Suspense>
    </div>
  );
}

// Nudges a fresh project toward its first host - a project with no hosts has nothing to
// show anywhere in the app, and the fix is always the same one action. Shown once per
// project (see useUIStore.emptyProjectPrompted), so switching away and back re-prompts
// while staying put doesn't. Same shape as ui/ConfirmModal, just with its own decline
// label; Continue drops the user on the hosts page rather than acting in place.
function GetStartedModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate();
  return (
    <Modal open={open} onClose={onClose} title='Add your first host'>
      <div className='space-y-4'>
        <p className='text-sm text-ink-dim'>
          Add your first host and this whole dashboard comes alive. It takes about a minute,
          and all you run is one command on the machine.
        </p>
        <div className='flex justify-end gap-2'>
          <Button variant='outline' onClick={onClose}>
            I'll do this later
          </Button>
          <Button
            onClick={() => {
              onClose();
              navigate("/hosts");
            }}
          >
            Continue
          </Button>
        </div>
      </div>
    </Modal>
  );
}

export function Dashboard() {
  usePageTitle("Overview");
  const queryClient = useQueryClient();
  const { activeProject } = useProjectStore();
  const projectKey = activeProject?.Key ?? "";

  const [alerts, setAlerts] = useState<ActiveAlert[]>([]);
  const [dismissed, setDismissed] = useState<Set<string>>(new Set());
  const { emptyProjectPrompted, markEmptyProjectPrompted } = useUIStore();

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

  // Same hostname -> color mapping for both the Cpu Usage and Memory Usage charts, so a
  // given machine's line is the same color on both. Assigned from a stable, alphabetized
  // host list rather than each chart's own Prometheus query result order, which can
  // differ between the two metrics and would otherwise put the same color on different
  // machines per chart. (Duplicated from DashboardCharts.tsx's own LINE_COLORS rather
  // than imported - that module pulls in recharts, and this needs to exist before that
  // chunk lazy-loads.)
  const hostColorMap = useMemo(() => {
    const LINE_COLORS = ["#3b82f6", "#22c55e", "#f59e0b", "#ef4444", "#a855f7", "#06b6d4", "#f97316", "#ec4899"];
    const names = Array.from(hostNameMap.values()).sort();
    const map: Record<string, string> = {};
    names.forEach((name, i) => {
      map[name] = LINE_COLORS[i % LINE_COLORS.length];
    });
    return map;
  }, [hostNameMap]);

  // Stable string key for queryKey (avoids array reference churn)
  const idKey = machineIds
    .slice()
    .sort((a, b) => a - b)
    .join(",");

  const promOk = prometheusService.isConfigured();
  const promEnabled = promOk && machineIds.length > 0;

  const qOpts = { refetchInterval: 5_000, staleTime: 0, enabled: promEnabled } as const;

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
  // Each chart carries its own window so two metrics can be compared over different
  // spans side by side (e.g. a 3-day CPU trend next to the last hour of throughput).
  const [throughputRange, setThroughputRange] = useState<RangeKey>("1h");
  const [memRange, setMemRange] = useState<RangeKey>("1h");
  const [cpuRange, setCpuRange] = useState<RangeKey>("1h");

  const { data: rThroughputHost } = useQuery({
    queryKey: ["d-tphost", idKey, throughputRange],
    queryFn: () => {
      const { start, end, step, rateWindow } = rangeWindow(throughputRange);
      return prometheusService.range(PQ.throughputPerHost(machineIds, rateWindow), start, end, step);
    },
    ...qOpts,
    refetchInterval: RANGES[throughputRange].refetchMs,
  });
  const { data: rCpuHost } = useQuery({
    queryKey: ["d-cpuhost", idKey, cpuRange],
    queryFn: () => {
      const { start, end, step, rateWindow } = rangeWindow(cpuRange);
      return prometheusService.range(PQ.cpuPerHost(machineIds, rateWindow), start, end, step);
    },
    ...qOpts,
    refetchInterval: RANGES[cpuRange].refetchMs,
  });
  const { data: rMemHost } = useQuery({
    queryKey: ["d-memhost", idKey, memRange],
    queryFn: () => {
      const { start, end, step } = rangeWindow(memRange);
      return prometheusService.range(PQ.memPerHost(machineIds), start, end, step);
    },
    ...qOpts,
    refetchInterval: RANGES[memRange].refetchMs,
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
  // True once we're not missing data the healthy count depends on - before this, some
  // health-checked apps would count as "down" simply because their status hasn't loaded
  // yet, understating the healthy count until it resolves (a misleading 0/N flash).
  const appStatusReady = !hcEnabled || hcStatusResult !== undefined;
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

  // Derived rather than held in local state, so dismissing it is the single act of
  // recording the prompt against this project key. `machines === undefined` means the
  // host query is still in flight, which must not be mistaken for an empty project.
  const showGetStarted =
    !!projectKey &&
    machines !== undefined &&
    machines.length === 0 &&
    emptyProjectPrompted !== projectKey;

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
  const netInVal = scalar(rNetIn);
  const netOutVal = scalar(rNetOut);

  const throughputPerfData = useMemo(
    () => toHostRows(rThroughputHost, hostNameMap, throughputRange),
    [rThroughputHost, hostNameMap, throughputRange],
  );
  const cpuPerfData = useMemo(
    () => toHostRows(rCpuHost, hostNameMap, cpuRange),
    [rCpuHost, hostNameMap, cpuRange],
  );
  const memPerfData = useMemo(
    () => toHostRows(rMemHost, hostNameMap, memRange),
    [rMemHost, hostNameMap, memRange],
  );

  const visibleAlerts = alerts.filter((a) => a.id && !dismissed.has(a.id));
  function dismiss(id: string) {
    setDismissed((prev) => new Set([...prev, id]));
  }

  return (
    <>
      <GetStartedModal
        open={showGetStarted}
        onClose={() => markEmptyProjectPrompted(projectKey)}
      />

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
        {/* ── Row 1: aggregate stat cards + per-host disk ─────────────── */}
        {/* Same grid-cols-3 as the charts row below, so the column boundaries line up
            between the two rows. The first third is a nested 2-up rather than four tracks
            in one grid: four tracks would carry three gaps against the charts row's two,
            and no ratio of fr units can reconcile that - the divisions would land a gap's
            worth apart no matter how they were tuned. */}
        <div className='grid grid-cols-3 gap-[14px]'>
          <div className='grid grid-cols-2 gap-[14px]'>
            <SevEventsCard count={sevLast24h} />
            <ApplicationStatusCard apps={dashboardApps} hcStatusMap={hcStatusMap} hcUptimeMap={hcUptimeMap} ready={appStatusReady} />
          </div>
          <BandwidthCard
            inVal={netInVal}
            outVal={netOutVal}
          />
          <DiskGauges machineIds={machineIds} hostNames={hostNameMap} />
        </div>

        {/* ── Row 2: Request Throughput + Cpu Usage + Memory Usage ──────── */}
        <div className='grid grid-cols-3 gap-[14px]'>
          <ChartCard
            title='Request Throughput'
            info='Live per-host IIS request rate.'
            range={throughputRange}
            onRangeChange={setThroughputRange}
          >
            <HostLineChart
              rows={throughputPerfData.rows}
              keys={throughputPerfData.keys}
              colors={hostColorMap}
              format='rate'
            />
          </ChartCard>

          <ChartCard
            title='Cpu Usage'
            info='Live per-host CPU usage.'
            range={cpuRange}
            onRangeChange={setCpuRange}
          >
            <HostLineChart rows={cpuPerfData.rows} keys={cpuPerfData.keys} colors={hostColorMap} />
          </ChartCard>

          <ChartCard
            title='Memory Usage'
            info='Live per-host memory usage.'
            range={memRange}
            onRangeChange={setMemRange}
          >
            <HostLineChart rows={memPerfData.rows} keys={memPerfData.keys} colors={hostColorMap} />
          </ChartCard>
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
