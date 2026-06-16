import { useState, useEffect, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { AlertTriangle, Monitor, Layers, Bell, X, ChevronRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Cell,
  ResponsiveContainer,
  PieChart,
  Pie,
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
  // % of last 24h that IIS (W3SVC) was in running state, averaged across all hosts
  serviceUptime: (ids: number[]) =>
    `avg(avg_over_time(windows_service_state{${mids(ids)},name="W3SVC",state="running"}[24h])) * 100`,

  // Total IIS request rate across all hosts and sites (req/s)
  throughput: (ids: number[]) => `sum(rate(windows_iis_requests_total{${mids(ids)}}[5m]))`,

  // Rejected requests as % of total - proxy for error rate until log tailing
  errorRate: (ids: number[]) =>
    `sum(rate(windows_iis_requests_rejected_total{${mids(
      ids,
    )}}[5m])) / sum(rate(windows_iis_requests_total{${mids(ids)}}[5m])) * 100`,

  // Average CPU % across all project hosts
  cpuAvg: (ids: number[]) =>
    `avg(100 - (avg by (machine_id) (rate(windows_cpu_time_total{${mids(ids)},mode="idle"}[5m])) * 100))`,

  // Average memory used % across all project hosts
  memAvg: (ids: number[]) =>
    `avg(100 - (windows_memory_physical_free_bytes{${mids(ids)}} / windows_memory_physical_total_bytes{${mids(
      ids,
    )}} * 100))`,

  // Total IIS request queue depth across all hosts
  queueDepth: (ids: number[]) => `sum(windows_iis_requests_queued{${mids(ids)}})`,

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

function fmtQueue(v: number) {
  return Math.round(v).toLocaleString();
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
  loading,
}: {
  label: string;
  value: string | null;
  sub?: string;
  loading: boolean;
}) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-5 py-5 flex flex-col gap-3'>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        {label}
      </span>
      <span
        className={cn(
          "text-3xl font-semibold font-mono leading-none",
          loading ? "text-ink-faint" : "text-ink",
        )}
      >
        {loading ? "-" : (value ?? "N/A")}
      </span>
      {sub && <span className='text-[0.625rem] text-ink-faint'>{sub}</span>}
    </div>
  );
}

// ── Radial gauge ──────────────────────────────────────────────────────────────

function RadialGauge({ label, value, loading }: { label: string; value: number | null; loading: boolean }) {
  const pct = Math.min(Math.max(value ?? 0, 0), 100);
  const color = pct >= 80 ? "#f85149" : pct >= 60 ? "#d29922" : "#3fb950";
  const data = [{ v: loading ? 0 : pct }, { v: loading ? 100 : 100 - pct }];

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
        {/* Value centered at the arc midpoint */}
        <div className='absolute inset-0 flex items-center justify-center' style={{ paddingTop: "10%" }}>
          <span className='text-2xl font-semibold font-mono text-ink leading-none'>
            {loading ? "-" : value != null ? fmtPct(value) : "N/A"}
          </span>
        </div>
      </div>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>{label}</span>
    </div>
  );
}

// ── Host performance chart ────────────────────────────────────────────────────

interface HostCpuRow {
  name: string;
  cpu: number;
}

function cpuBarColor(v: number) {
  if (v >= 80) return "#f85149"; // danger
  if (v >= 60) return "#d29922"; // warning
  return "#3fb950"; // success
}

const TICK_STYLE = { fill: "#8b949e", fontSize: 10 };

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function HostTooltipContent({ active, payload, label }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div className='rounded-md border border-rim bg-surface-highest px-3 py-2 text-xs shadow-xl'>
      <p className='font-mono text-ink-faint mb-0.5'>{label}</p>
      <p className='font-mono font-semibold text-ink'>{(payload[0].value as number).toFixed(1)}%</p>
    </div>
  );
}

function HostPerfChart({ data, loading }: { data: HostCpuRow[]; loading: boolean }) {
  if (loading) {
    return (
      <div className='flex flex-1 items-center justify-center py-8'>
        <span className='size-4 rounded-full border-2 border-primary border-t-transparent animate-spin' />
      </div>
    );
  }
  if (!data.length) {
    return (
      <div className='flex flex-1 items-center justify-center py-8'>
        <p className='text-xs text-ink-faint'>No data</p>
      </div>
    );
  }

  const chartHeight = Math.max(120, data.length * 48 + 32);

  return (
    <ResponsiveContainer width='100%' height={chartHeight}>
      <BarChart
        layout='vertical'
        data={data}
        barCategoryGap='15%'
        margin={{ left: 0, right: 32, top: 16, bottom: 16 }}
      >
        <CartesianGrid
          horizontal={false}
          verticalValues={[25, 50, 75]}
          stroke='#30363d'
          strokeDasharray='3 3'
        />
        <XAxis
          type='number'
          domain={[0, 100]}
          ticks={[0, 25, 50, 75, 100]}
          tick={TICK_STYLE}
          axisLine={{ stroke: "#2a2a2a" }}
          tickLine={{ stroke: "#2a2a2a" }}
          tickFormatter={(v) => `${v}%`}
        />
        <YAxis
          type='category'
          dataKey='name'
          tick={TICK_STYLE}
          axisLine={{ stroke: "#2a2a2a" }}
          tickLine={false}
          width={100}
        />
        <Tooltip content={HostTooltipContent} cursor={{ fill: "rgba(255,255,255,0.04)" }} />
        <Bar dataKey='cpu' radius={[0, 3, 3, 0]} maxBarSize={18}>
          {data.map((entry, i) => (
            <Cell key={i} fill={cpuBarColor(entry.cpu)} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}

// ── Notification row ──────────────────────────────────────────────────────────

const NOTIF_DOT: Record<string, string> = {
  error: "bg-danger",
  warning: "bg-warning",
  info: "bg-[#2f81f7]",
};

function NotifRow({ notif }: { notif: DashboardNotification }) {
  const Icon = categoryIcon(notif.Category);
  return (
    <div className='flex items-center gap-2.5 py-2 border-b border-rim last:border-0'>
      <span
        className={cn("size-1.5 rounded-full shrink-0", NOTIF_DOT[notif.Level ?? ""] ?? "bg-ink-faint")}
      />
      <Icon className='size-3.5 text-ink-faint shrink-0' />
      <div className='flex-1 min-w-0'>
        <p className='text-xs text-ink truncate'>{notif.Title ?? ""}</p>
        {notif.Detail ? <p className='text-[0.6875rem] text-ink-faint truncate'>{notif.Detail}</p> : null}
      </div>
      <span className='text-xs text-ink-faint shrink-0 w-14 text-right'>{timeAgo(notif.CreatedAt)}</span>
    </div>
  );
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export function Dashboard() {
  usePageTitle("Dashboard");
  const queryClient = useQueryClient();
  const { activeProject } = useProjectStore();
  const greeting = "Welcome Back";
  const projectKey = activeProject?.Key ?? "";

  const [alerts, setAlerts] = useState<ActiveAlert[]>([]);
  const [dismissed, setDismissed] = useState<Set<string>>(new Set());

  // ── Fetch project machines to build Prometheus label filter ───────────────

  const { data: machinesResp } = useQuery({
    queryKey: ["machines", projectKey],
    queryFn: () => machineService.list(projectKey),
    enabled: !!projectKey,
    staleTime: 60_000,
  });

  const machineIds = useMemo(() => (machinesResp?.data?.machines ?? []).map((m) => m.ID), [machinesResp]);

  // Map machine_id → hostname for the performance chart labels
  const hostNameMap = useMemo(() => {
    const map = new Map<number, string>();
    for (const m of machinesResp?.data?.machines ?? []) {
      map.set(m.ID, m.Hostname?.toLowerCase() ?? String(m.ID));
    }
    return map;
  }, [machinesResp]);

  // Stable string key for queryKey (avoids array reference churn)
  const idKey = machineIds
    .slice()
    .sort((a, b) => a - b)
    .join(",");

  const promOk = prometheusService.isConfigured();
  const promEnabled = promOk && machineIds.length > 0;

  const qOpts = { refetchInterval: 5_000, staleTime: 0, enabled: promEnabled } as const;

  // ── Prometheus instant queries (all project-level aggregates) ─────────────

  const { data: rUptime, isLoading: lUptime } = useQuery({
    queryKey: ["d-uptime", idKey],
    queryFn: () => prometheusService.instant(PQ.serviceUptime(machineIds)),
    ...qOpts,
  });
  const { data: rThroughput, isLoading: lThroughput } = useQuery({
    queryKey: ["d-throughput", idKey],
    queryFn: () => prometheusService.instant(PQ.throughput(machineIds)),
    ...qOpts,
  });
  const { data: rError, isLoading: lError } = useQuery({
    queryKey: ["d-error", idKey],
    queryFn: () => prometheusService.instant(PQ.errorRate(machineIds)),
    ...qOpts,
  });
  const { data: rCpuAvg, isLoading: lCpuAvg } = useQuery({
    queryKey: ["d-cpuavg", idKey],
    queryFn: () => prometheusService.instant(PQ.cpuAvg(machineIds)),
    ...qOpts,
  });
  const { data: rMemAvg, isLoading: lMemAvg } = useQuery({
    queryKey: ["d-memavg", idKey],
    queryFn: () => prometheusService.instant(PQ.memAvg(machineIds)),
    ...qOpts,
  });
  const { data: rQueue, isLoading: lQueue } = useQuery({
    queryKey: ["d-queue", idKey],
    queryFn: () => prometheusService.instant(PQ.queueDepth(machineIds)),
    ...qOpts,
  });
  const { data: rCpuHost, isLoading: lCpuHost } = useQuery({
    queryKey: ["d-cpuhost", idKey],
    queryFn: () => prometheusService.instant(PQ.cpuPerHost(machineIds)),
    ...qOpts,
  });

  // ── Notifications ─────────────────────────────────────────────────────────

  const { data: notifications = [] } = useQuery({
    queryKey: ["dashboard-notifications"],
    queryFn: () => dashboardService.getNotifications(),
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
              const n = msg.payload as DashboardNotification;
              queryClient.setQueryData(
                ["dashboard-notifications"],
                (old: DashboardNotification[] | undefined) => [n, ...(old ?? [])].slice(0, 10),
              );
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
  }, [queryClient]);

  // ── Derived values ────────────────────────────────────────────────────────

  const uptimeVal = scalar(rUptime);
  const throughputVal = scalar(rThroughput);
  const errorVal = scalar(rError);
  const cpuAvgVal = scalar(rCpuAvg);
  const memAvgVal = scalar(rMemAvg);
  const queueVal = scalar(rQueue);

  // Per-host CPU data for bar chart - sorted worst (highest %) first
  const hostPerfData = useMemo<HostCpuRow[]>(() => {
    if (!rCpuHost?.length) return [];
    return rCpuHost
      .map((r) => ({
        name: hostNameMap.get(Number(r.metric.machine_id)) ?? r.metric.machine_id ?? "?",
        cpu: parseFloat(r.value[1]),
      }))
      .filter((r) => isFinite(r.cpu))
      .sort((a, b) => b.cpu - a.cpu);
  }, [rCpuHost, hostNameMap]);

  const visibleAlerts = alerts.filter((a) => a.id && !dismissed.has(a.id));
  function dismiss(id: string) {
    setDismissed((prev) => new Set([...prev, id]));
  }

  // loading = only show spinner when Prometheus is enabled AND query is in-flight
  function pl(l: boolean) {
    return promEnabled && l;
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

      {/* Greeting */}
      <div className='mb-6'>
        <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mb-2'>
          {activeProject?.Name ?? "Dashboard"}
        </p>
        <h1 className='text-2xl font-semibold text-ink tracking-tight'>{greeting}</h1>
      </div>

      <div className='space-y-4'>
        {/* ── Row 1: 4 aggregate stat cards ───────────────────────────── */}
        <div className='grid grid-cols-4 gap-4'>
          <MetricCard
            label='Service Uptime'
            value={uptimeVal != null ? fmtPct(uptimeVal) : null}
            sub='24h average across hosts'
            loading={pl(lUptime)}
          />
          <MetricCard
            label='Request Throughput'
            value={throughputVal != null ? fmtRate(throughputVal) : null}
            sub='last 5 min'
            loading={pl(lThroughput)}
          />
          <MetricCard
            label='Error Rate'
            value={errorVal != null ? fmtPct(errorVal) : null}
            sub='rejected / total requests'
            loading={pl(lError)}
          />
          <MetricCard
            label='Queue Depth'
            value={queueVal != null ? fmtQueue(queueVal) : null}
            sub='current queued requests'
            loading={pl(lQueue)}
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
              <RadialGauge label='CPU Avg' value={cpuAvgVal} loading={pl(lCpuAvg)} />
              <RadialGauge label='Memory Avg' value={memAvgVal} loading={pl(lMemAvg)} />
            </div>
          </div>

          {/* Host Performance - per-host CPU sorted worst → best */}
          <div className='col-span-3 rounded-lg border border-rim bg-surface p-4 flex flex-col gap-3'>
            <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Host Performance CPU %
            </p>
            <HostPerfChart data={hostPerfData} loading={pl(lCpuHost)} />
          </div>
        </div>

        {/* ── Row 3: Recent Activity ───────────────────────────────────── */}
        <div className='rounded-lg border border-rim bg-surface px-4 pt-4 pb-2'>
          <div className='flex items-center justify-between mb-3 pr-1'>
            <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Recent Activity
            </p>
            <Link
              to='/alerts'
              className='flex items-center gap-1 text-xs text-ink-faint hover:text-ink transition-colors'
            >
              View All <ChevronRight className='size-3.5' />
            </Link>
          </div>
          {notifications.length > 0 ? (
            <div className='max-h-72 overflow-y-auto pr-3'>
              {notifications.map((n, i) => (
                <NotifRow key={n.ID ?? i} notif={n} />
              ))}
            </div>
          ) : (
            <div className='flex h-24 items-center justify-center text-xs text-ink-faint'>
              No recent activity
            </div>
          )}
        </div>
      </div>
    </>
  );
}
