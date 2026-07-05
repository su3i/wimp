import { useState, useEffect, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { AlertTriangle, Monitor, Layers, Bell, X, ChevronRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Cell,
  ResponsiveContainer,
  PieChart,
  Pie,
  Legend,
} from "recharts";
import { useAuthStore } from "@/store/auth";
import { useProjectStore } from "@/store/project";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";
import { machineService } from "@/services/machine.service";
import { prometheusService, type PromInstantResult } from "@/services/prometheus.service";
import { dashboardService, type ActiveAlert, type DashboardNotification } from "@/services/dashboard.service";

// ── Prometheus query builders ─────────────────────────────────────────────────

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

// ── Formatters ────────────────────────────────────────────────────────────────

function fmtPct(v: number) {
  return `${v.toFixed(1)}%`;
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

function timeAgo(dateStr: string | null | undefined): string {
  if (!dateStr) return "Never";
  try {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60_000);
    if (mins < 1) return "Just now";
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  } catch {
    return "N/A";
  }
}

// ── Icon helpers ──────────────────────────────────────────────────────────────

function categoryIcon(cat: string | null | undefined): LucideIcon {
  if (cat === "machine") return Monitor;
  if (cat === "app_pool") return Layers;
  return Bell;
}

function alertIcon(cat: string | null | undefined): LucideIcon {
  if (cat === "machine") return Monitor;
  if (cat === "app_pool") return Layers;
  return AlertTriangle;
}

// ── Alert banner row ──────────────────────────────────────────────────────────

function AlertRow({ alert, onDismiss }: { alert: ActiveAlert; onDismiss: () => void }) {
  const Icon = alertIcon(alert.category);
  return (
    <div className='flex items-center gap-3 px-4 py-2.5 bg-danger text-white border-b border-red-700/40 last:border-0'>
      <span className='shrink-0 text-base'>🚨</span>
      <Icon className='size-4 shrink-0 opacity-75' />
      <span className='flex-1 text-sm'>{alert.message ?? "Alert"}</span>
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

// ── Metric stat card ──────────────────────────────────────────────────────────

function MetricCard({
  label,
  value,
  sub,
}: {
  label: string;
  value: string | null;
  sub?: string;
}) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-5 py-5 flex flex-col gap-3'>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        {label}
      </span>
      <span className='text-3xl font-semibold font-mono leading-none text-ink'>
        {value ?? "N/A"}
      </span>
      {sub && <span className='text-[0.625rem] text-ink-faint'>{sub}</span>}
    </div>
  );
}

// ── Critical events card ──────────────────────────────────────────────────────

function CriticalEventsCard({ count }: { count: number }) {
  const hot = count > 0;
  return (
    <div className={cn(
      'rounded-lg border bg-surface px-5 py-5 flex flex-col gap-3 transition-colors border-rim',
    )}>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        Critical Events
      </span>
      <span className={cn('text-3xl font-semibold font-mono leading-none', hot ? 'text-danger' : 'text-ink')}>
        {count}
      </span>
      <span className='text-[0.625rem] text-ink-faint'>last hour</span>
    </div>
  );
}

// ── Bandwidth card ────────────────────────────────────────────────────────────

function BandwidthCard({ inVal, outVal }: { inVal: number | null; outVal: number | null }) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-5 py-5 flex flex-col gap-3'>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        Bandwidth
      </span>
      <div className='flex items-end gap-4'>
        <span className='text-2xl font-semibold font-mono leading-none text-ink'>
          {inVal != null ? fmtBytes(inVal) : 'N/A'}
          <span className='ml-1.5 text-[0.625rem] text-ink-faint'>IN</span>
        </span>
        <span className='text-ink-faint/40 text-lg font-light mb-0.5'>/</span>
        <span className='text-2xl font-semibold font-mono leading-none text-ink'>
          {outVal != null ? fmtBytes(outVal) : 'N/A'}
          <span className='ml-1.5 text-[0.625rem] text-ink-faint'>OUT</span>
        </span>
      </div>
      <span className='text-[0.625rem] text-ink-faint'>last 5 min</span>
    </div>
  );
}

// ── Radial gauge ──────────────────────────────────────────────────────────────

function RadialGauge({ label, value }: { label: string; value: number | null }) {
  const pct = Math.min(Math.max(value ?? 0, 0), 100);
  const color = pct >= 80 ? "#f85149" : pct >= 60 ? "#d29922" : "#3fb950";
  const data = [{ v: pct }, { v: 100 - pct }];

  return (
    <div className='flex flex-col items-center'>
      <div className='relative w-full' style={{ height: 180 }}>
        <ResponsiveContainer width='100%' height='100%'>
          <PieChart margin={{ top: 0, right: 0, bottom: 0, left: 0 }}>
            <Pie
              data={data}
              dataKey='v'
              cx='50%'
              cy='55%'
              startAngle={220}
              endAngle={-40}
              innerRadius='58%'
              outerRadius='80%'
              strokeWidth={0}
              isAnimationActive={false}
            >
              <Cell fill={color} />
              <Cell fill='#21262d' />
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className='absolute inset-0 flex items-center justify-center' style={{ paddingTop: "10%" }}>
          <span className='text-2xl font-semibold font-mono text-ink leading-none'>
            {value != null ? fmtPct(value) : "N/A"}
          </span>
        </div>
      </div>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>{label}</span>
    </div>
  );
}

// ── Host CPU line chart ───────────────────────────────────────────────────────

const TICK_STYLE = { fill: "#8b949e", fontSize: 10 };
const LINE_COLORS = ["#3b82f6", "#22c55e", "#f59e0b", "#ef4444", "#a855f7", "#06b6d4", "#f97316", "#ec4899"];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function HostTooltipContent({ active, payload, label }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div className='rounded-md border border-rim bg-surface-highest px-3 py-2 text-xs shadow-xl min-w-[150px]'>
      <p className='font-mono text-ink-faint mb-1.5'>{label}</p>
      {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
      {payload.map((p: any) => (
        <div key={p.name} className='flex items-center gap-2 py-0.5'>
          <span className='size-1.5 rounded-full shrink-0' style={{ background: p.color }} />
          <span className='text-ink-dim flex-1 truncate'>{p.name}</span>
          <span className='font-mono font-semibold text-ink ml-2'>{(p.value as number).toFixed(1)}%</span>
        </div>
      ))}
    </div>
  );
}

function HostCpuLineChart({
  rows,
  keys,
}: {
  rows: Record<string, string | number>[];
  keys: string[];
}) {
  if (!rows.length || !keys.length) {
    return (
      <div className='flex flex-1 items-center justify-center py-8'>
        <p className='text-xs text-ink-faint'>No data</p>
      </div>
    );
  }

  return (
    <ResponsiveContainer width='100%' height={260}>
      <LineChart data={rows} margin={{ left: 0, right: 8, top: 8, bottom: 0 }}>
        <CartesianGrid stroke='#30363d' strokeDasharray='3 3' vertical={false} />
        <XAxis
          dataKey='time'
          tick={TICK_STYLE}
          axisLine={false}
          tickLine={false}
          interval='preserveStartEnd'
        />
        <YAxis
          domain={[0, 100]}
          tick={TICK_STYLE}
          axisLine={false}
          tickLine={false}
          tickFormatter={(v) => `${v}%`}
          width={36}
        />
        <Tooltip content={HostTooltipContent} />
        <Legend wrapperStyle={{ fontSize: 10, color: "#8b949e" }} />
        {keys.map((key, i) => (
          <Line
            key={key}
            type='monotone'
            dataKey={key}
            stroke={LINE_COLORS[i % LINE_COLORS.length]}
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}

// ── Recent alerts table ───────────────────────────────────────────────────────

const LEVEL_CFG: Record<string, { label: string; cls: string }> = {
  critical: {
    label: "Critical",
    cls: "bg-danger/10 text-danger border border-danger/20",
  },
  warning: {
    label: "Warning",
    cls: "bg-[#d29922]/10 text-[#d29922] border border-[#d29922]/20",
  },
  info: {
    label: "Info",
    cls: "bg-surface-high text-ink-faint border border-rim",
  },
};

function LevelBadge({ level }: { level: string }) {
  const cfg = LEVEL_CFG[level] ?? { label: level, cls: "bg-surface-high text-ink-faint border border-rim" };
  return (
    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-[0.625rem] font-semibold uppercase tracking-wide whitespace-nowrap", cfg.cls)}>
      {cfg.label}
    </span>
  );
}

function categoryLabel(cat: string | null | undefined): string {
  if (cat === "machine") return "Machine";
  if (cat === "apppool" || cat === "app_pool") return "App Pool";
  if (cat === "iis") return "IIS";
  if (cat === "service") return "Service";
  return cat ?? "System";
}

function NotifRow({ notif }: { notif: DashboardNotification }) {
  const CatIcon = categoryIcon(notif.Category);
  return (
    <div className='grid grid-cols-[84px_1fr_124px_92px] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'>
      <div className='px-4 py-2.5'>
        <LevelBadge level={notif.Level ?? "info"} />
      </div>
      <div className='px-4 py-2.5 min-w-0'>
        <p className='text-xs font-medium text-ink truncate'>{notif.Title ?? ""}</p>
        {notif.Detail ? (
          <p className='mt-0.5 text-[0.6875rem] text-ink-faint truncate'>{notif.Detail}</p>
        ) : null}
      </div>
      <div className='flex items-center gap-1.5 px-4 py-2.5'>
        <CatIcon className='size-3 text-ink-faint shrink-0' />
        <span className='text-xs text-ink-dim'>{categoryLabel(notif.Category)}</span>
      </div>
      <div className='px-4 py-2.5 text-right'>
        <span className='text-xs text-ink-faint tabular-nums'>{timeAgo(notif.CreatedAt)}</span>
      </div>
    </div>
  );
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export function Dashboard() {
  usePageTitle("Dashboard");
  const queryClient = useQueryClient();
  const { activeProject } = useProjectStore();
  const projectKey = activeProject?.Key ?? "";

  const [alerts, setAlerts] = useState<ActiveAlert[]>([]);
  const [dismissed, setDismissed] = useState<Set<string>>(new Set());

  // ── Fetch project machines to build Prometheus label filter ───────────────

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

  // Map machine_id → hostname for the performance chart labels
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

  // ── Prometheus instant queries (all project-level aggregates) ─────────────

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

  // ── Dashboard stats (machines count + critical_last_hour) ─────────────────

  const { data: stats } = useQuery({
    queryKey: ["dashboard-stats", projectKey],
    queryFn: () => dashboardService.getStats(projectKey!),
    enabled: !!projectKey,
    refetchInterval: 30_000,
    staleTime: 0,
  });

  // ── Notifications ─────────────────────────────────────────────────────────

  const { data: notifications = [] } = useQuery({
    queryKey: ["dashboard-notifications", projectKey],
    queryFn: () => dashboardService.getNotifications(projectKey || undefined),
    enabled: !!projectKey,
    refetchInterval: 5_000,
    select: (d) => d ?? [],
  });

  // ── Initial alert load ────────────────────────────────────────────────────

  useEffect(() => {
    void dashboardService.getActiveAlerts().then((d) => setAlerts(d ?? []));
  }, []);

  // ── WebSocket (alerts + live notifications) ───────────────────────────────

  useEffect(() => {
    const { accessToken } = useAuthStore.getState();
    if (!accessToken) return;

    const base = (import.meta.env.VITE_API_BASE_URL as string)
      .replace(/\/api\/v1.*$/, "")
      .replace(/^http/, "ws");

    let ws: WebSocket | null = null;
    try {
      ws = new WebSocket(`${base}/ws?token=${encodeURIComponent(accessToken)}`);
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
    } catch {
      /* WS not yet available */
    }

    return () => {
      ws?.close();
    };
  }, [queryClient, projectKey]);

  // ── Derived values ────────────────────────────────────────────────────────

  const criticalLastHour = stats?.critical_last_hour ?? 0;
  const throughputVal = scalar(rThroughput);
  const cpuAvgVal = scalar(rCpuAvg);
  const memAvgVal = scalar(rMemAvg);
  const netInVal = scalar(rNetIn);
  const netOutVal = scalar(rNetOut);

  // Per-host CPU time-series — one row per timestamp, one key per machine
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

  // ── Render ────────────────────────────────────────────────────────────────

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
      <div className='mb-6'>
        <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mb-2'>Overview</p>
        <h1 className='text-2xl font-semibold text-ink tracking-tight'>{activeProject?.Name ?? "Dashboard"}</h1>
      </div>

      <div className='space-y-4'>
        {/* ── Row 1: 4 aggregate stat cards ───────────────────────────── */}
        <div className='grid grid-cols-[1fr_1fr_1.5fr] gap-4'>
          <CriticalEventsCard count={criticalLastHour} />
          <MetricCard
            label='Request Throughput'
            value={throughputVal != null ? fmtRate(throughputVal) : null}
            sub='last 5 min'
          />
          <BandwidthCard
            inVal={netInVal}
            outVal={netOutVal}
          />
        </div>

        {/* ── Row 2: Capacity Overview + Host Performance ──────────────── */}
        <div className='grid grid-cols-5 gap-4'>
          {/* Capacity Overview */}
          <div className='col-span-2 rounded-lg border border-rim bg-surface p-4 flex flex-col gap-4'>
            <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Capacity Overview
            </p>
            <div className='grid grid-cols-2 gap-2 flex-1'>
              <RadialGauge label='CPU Avg' value={cpuAvgVal} />
              <RadialGauge label='Memory Avg' value={memAvgVal} />
            </div>
          </div>

          {/* Host Performance — per-host CPU time series */}
          <div className='col-span-3 rounded-lg border border-rim bg-surface p-4 flex flex-col gap-3'>
            <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Host CPU %
            </p>
            <HostCpuLineChart rows={hostPerfData.rows} keys={hostPerfData.keys} />
          </div>
        </div>

        {/* ── Row 3: Recent Alerts ─────────────────────────────────────── */}
        <div className='rounded-lg border border-rim overflow-hidden'>
          <div className='flex items-center justify-between px-4 py-3 border-b border-rim bg-surface-alt'>
            <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Recent Alerts
            </p>
            <Link
              to='/alerts'
              className='flex items-center gap-1 text-xs text-ink-faint hover:text-ink transition-colors'
            >
              View All <ChevronRight className='size-3.5' />
            </Link>
          </div>
          {notifications.length > 0 ? (
            <div className='max-h-72 overflow-y-auto bg-surface'>
              {notifications.map((n, i) => (
                <NotifRow key={n.ID ?? i} notif={n} />
              ))}
            </div>
          ) : (
            <div className='flex h-24 items-center justify-center text-xs text-ink-faint bg-surface'>
              No recent alerts
            </div>
          )}
        </div>
      </div>
    </>
  );
}
