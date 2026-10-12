import { useQuery } from "@tanstack/react-query";
import { Area, AreaChart, Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Activity } from "lucide-react";
import { prometheusService, type PromRangeResult } from "@/services/prometheus.service";
import { formatUptime } from "@/utils/time";
import { promSelector, type SqlInstanceGroup } from "@/utils/sql";

// Same palette, axes, tooltip and card shell as the host Overview, so the two read alike.
const C = { blue: "#3b82f6", green: "#22c55e", amber: "#f59e0b", red: "#ef4444", purple: "#a855f7", cyan: "#06b6d4" };
const TICK = { fill: "#8c909f", fontSize: 10 };
const LEGEND_STYLE = { fontSize: 10, color: "#8c909f" };
const REFETCH_MS = 30_000;
const RANGE_SECS = 30 * 60;
const STEP = 60;

const now = () => Math.floor(Date.now() / 1000);
const fmtTime = (ts: number) => new Date(ts * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
const num = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(1)}k` : v >= 10 ? v.toFixed(0) : v.toFixed(1));

function DarkTooltip(fmt: (v: number) => string) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return function TooltipContent({ active, payload, label }: any) {
    if (!active || !payload?.length) return null;
    return (
      <div className='rounded-md border border-rim bg-surface-highest px-3 py-2 text-xs shadow-xl min-w-[120px]'>
        <p className='text-ink-faint mb-1.5 tabular-nums'>{label}</p>
        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
        {payload.map((p: any) => (
          <div key={p.name} className='flex items-center gap-2 py-0.5'>
            <span className='size-1.5 rounded-full shrink-0' style={{ background: p.color }} />
            <span className='text-ink-dim'>{p.name}</span>
            <span className='text-ink tabular-nums ml-auto pl-3'>{fmt(p.value)}</span>
          </div>
        ))}
      </div>
    );
  };
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-5 py-5'>
      <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mb-2.5'>{label}</p>
      <p className='text-2xl font-semibold text-ink tabular-nums leading-none'>{value}</p>
      {sub && <p className='text-[0.625rem] text-ink-faint mt-1.5'>{sub}</p>}
    </div>
  );
}

function Card({ title, sub, loading, empty, children }: { title: string; sub?: string; loading?: boolean; empty?: boolean; children: React.ReactNode }) {
  return (
    <div className='rounded-lg border border-rim bg-surface p-4'>
      <div className='mb-4 flex items-baseline justify-between gap-3'>
        <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>{title}</p>
        {sub && <p className='text-[0.625rem] text-ink-faint'>{sub}</p>}
      </div>
      {loading ? (
        <div className='h-40 flex items-center justify-center'>
          <span className='size-4 rounded-full border-2 border-primary border-t-transparent animate-spin' />
        </div>
      ) : empty ? (
        <div className='h-40 flex items-center justify-center'>
          <p className='text-xs text-ink-faint'>No data</p>
        </div>
      ) : (
        children
      )}
    </div>
  );
}

interface Series {
  label: string;
  color: string;
  query: string;
}

// One area chart over the last 30 minutes. Each series is its own range query, merged on
// timestamp so series with gaps still line up.
function TrendChart({ id, title, sub, series, fmt }: { id: string; title: string; sub?: string; series: Series[]; fmt: (v: number) => string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["sql-trend", id, ...series.map((s) => s.query)],
    refetchInterval: REFETCH_MS,
    queryFn: async () => {
      const n = now();
      const results = await Promise.all(
        series.map((s) => prometheusService.range(s.query, n - RANGE_SECS, n, STEP).catch(() => [] as PromRangeResult[])),
      );
      const rows = new Map<number, Record<string, string | number>>();
      results.forEach((r, i) => {
        for (const [ts, v] of r[0]?.values ?? []) {
          const row = rows.get(ts) ?? { time: fmtTime(ts) };
          row[series[i].label] = parseFloat(v);
          rows.set(ts, row);
        }
      });
      return [...rows.entries()].sort((a, b) => a[0] - b[0]).map(([, row]) => row);
    },
  });

  return (
    <Card title={title} sub={sub} loading={isLoading} empty={!data?.length}>
      <ResponsiveContainer width='100%' height={160}>
        <AreaChart data={data}>
          <defs>
            {series.map((s, i) => (
              <linearGradient key={i} id={`g-${id}-${i}`} x1='0' y1='0' x2='0' y2='1'>
                <stop offset='5%' stopColor={s.color} stopOpacity={0.18} />
                <stop offset='95%' stopColor={s.color} stopOpacity={0} />
              </linearGradient>
            ))}
          </defs>
          <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' vertical={false} />
          <XAxis dataKey='time' tick={TICK} axisLine={false} tickLine={false} interval='preserveStartEnd' />
          <YAxis tick={TICK} axisLine={false} tickLine={false} tickFormatter={fmt} width={44} />
          <Tooltip content={DarkTooltip(fmt)} />
          {series.length > 1 && <Legend wrapperStyle={LEGEND_STYLE} />}
          {series.map((s, i) => (
            <Area key={s.label} type='monotone' dataKey={s.label} name={s.label} stroke={s.color} fill={`url(#g-${id}-${i})`} strokeWidth={1.5} dot={false} />
          ))}
        </AreaChart>
      </ResponsiveContainer>
    </Card>
  );
}

function instant(query: string) {
  return prometheusService.instant(query).then((r) => {
    const v = Number(r[0]?.value?.[1]);
    return Number.isFinite(v) ? v : null;
  });
}

export function SqlOverviewTab({ group }: { group: SqlInstanceGroup }) {
  if (!prometheusService.isConfigured()) {
    return (
      <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
        <Activity className='size-5 text-ink-faint mb-3' />
        <p className='text-sm font-semibold text-ink'>Metrics not configured</p>
      </div>
    );
  }

  // Performance counters come from windows_exporter (mssql_instance); DMV metrics come from the
  // agent (sql_instance) and need SQL read access.
  const a = promSelector(group, "mssql_instance");
  const b = promSelector(group, "sql_instance");

  return <OverviewBody a={a} b={b} id={group.key} />;
}

function OverviewBody({ a, b, id }: { a: string; b: string; id: string }) {
  const stats = useQuery({
    queryKey: ["sql-stats", id],
    refetchInterval: REFETCH_MS,
    queryFn: async () => {
      const [batches, conns, ple, uptime] = await Promise.all([
        instant(`sum(rate(windows_mssql_sqlstats_batch_requests{${a}}[5m]))`),
        instant(`sum(windows_mssql_genstats_user_connections{${a}})`),
        instant(`min(windows_mssql_bufman_page_life_expectancy_seconds{${a}})`),
        instant(`max(wimp_mssql_uptime_seconds{${b}})`),
      ]);
      return { batches, conns, ple, uptime };
    },
  });

  const waits = useQuery({
    queryKey: ["sql-waits", id],
    refetchInterval: REFETCH_MS,
    queryFn: async () =>
      (await prometheusService.instant(`topk(8, sum by (wait_type) (rate(wimp_mssql_wait_seconds_total{${b}}[30m])))`))
        .map((r) => ({ type: r.metric.wait_type ?? "?", seconds: parseFloat(r.value[1]) }))
        .filter((w) => w.seconds > 0)
        .sort((x, y) => y.seconds - x.seconds),
  });

  const s = stats.data;
  const v = (n: number | null | undefined, f: (n: number) => string) => (n != null ? f(n) : "N/A");

  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-4 gap-3'>
        <StatCard label='Batch Requests' value={v(s?.batches, num)} sub='per second, last 5 min' />
        <StatCard label='User Connections' value={v(s?.conns, (n) => String(Math.round(n)))} />
        <StatCard
          label='Page Life Expectancy'
          value={v(s?.ple, (n) => formatUptime(n))}
          sub='how long a page stays in memory'
        />
        <StatCard label='Uptime' value={v(s?.uptime, formatUptime)} sub={s && s.uptime == null ? 'needs SQL read access' : undefined} />
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <TrendChart
          id={`${id}-batches`}
          title='Batch Requests / sec'
          series={[
            { label: "Batches", color: C.blue, query: `sum(rate(windows_mssql_sqlstats_batch_requests{${a}}[5m]))` },
            { label: "Compilations", color: C.amber, query: `sum(rate(windows_mssql_sqlstats_sql_compilations{${a}}[5m]))` },
          ]}
          fmt={num}
        />
        <TrendChart
          id={`${id}-tx`}
          title='Transactions / sec'
          series={[{ label: "Transactions", color: C.green, query: `sum(rate(windows_mssql_databases_transactions{${a},database!="_Total"}[5m]))` }]}
          fmt={num}
        />
        <TrendChart
          id={`${id}-conns`}
          title='User Connections'
          series={[{ label: "Connections", color: C.cyan, query: `sum(windows_mssql_genstats_user_connections{${a}})` }]}
          fmt={(n) => String(Math.round(n))}
        />
        <TrendChart
          id={`${id}-ple`}
          title='Page Life Expectancy'
          sub='a steady drop means memory pressure'
          series={[{ label: "Seconds", color: C.purple, query: `min(windows_mssql_bufman_page_life_expectancy_seconds{${a}})` }]}
          fmt={num}
        />
        <TrendChart
          id={`${id}-blocking`}
          title='Blocking & Deadlocks'
          series={[
            { label: "Blocked processes", color: C.amber, query: `sum(windows_mssql_genstats_blocked_processes{${a}})` },
            { label: "Deadlocks / min", color: C.red, query: `sum(rate(windows_mssql_locks_deadlocks{${a}}[5m])) * 60` },
          ]}
          fmt={num}
        />
        <TrendChart
          id={`${id}-grants`}
          title='Memory Grants Pending'
          sub='queries waiting for memory to run'
          series={[{ label: "Pending", color: C.red, query: `sum(windows_mssql_memmgr_pending_memory_grants{${a}})` }]}
          fmt={(n) => String(Math.round(n))}
        />
      </div>

      <Card
        title='Top Waits'
        sub='seconds waited per second, last 30 min'
        loading={waits.isLoading}
        empty={!waits.data?.length}
      >
        <ResponsiveContainer width='100%' height={Math.max(80, (waits.data?.length ?? 0) * 30)}>
          <BarChart data={waits.data} layout='vertical' margin={{ left: 0, right: 16 }}>
            <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' horizontal={false} />
            <XAxis type='number' tick={TICK} axisLine={false} tickLine={false} tickFormatter={num} />
            <YAxis type='category' dataKey='type' tick={TICK} axisLine={false} tickLine={false} width={190} />
            <Tooltip content={DarkTooltip((n) => `${n.toFixed(2)} s/s`)} />
            <Bar dataKey='seconds' name='Waited' fill={C.blue} radius={[0, 3, 3, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </Card>
    </div>
  );
}
