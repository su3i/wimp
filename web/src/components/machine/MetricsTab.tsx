import { type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  AreaChart,
  Area,
  LineChart,
  Line,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import { Activity } from "lucide-react";
import {
  prometheusService,
  type PromRangeResult,
  type PromInstantResult,
} from "@/services/prometheus.service";

// ── Theme colors (keep in sync with theme.ts) ─────────────────────────────────

const C = {
  blue: "#3b82f6",
  green: "#22c55e",
  amber: "#f59e0b",
  red: "#ef4444",
  purple: "#a855f7",
  cyan: "#06b6d4",
};

const MULTI_PALETTE = [C.blue, C.green, C.amber, C.red, C.purple, C.cyan];

// ── Prometheus query builders ─────────────────────────────────────────────────

function mid(machineId: number) {
  return `machine_id="${machineId}"`;
}

const Q = {
  cpuTotal: (id: number) =>
    `100 - (avg by (hostname) (rate(windows_cpu_time_total{${mid(id)},mode="idle"}[5m])) * 100)`,
  cpuPerCore: (id: number) => `100 - (rate(windows_cpu_time_total{${mid(id)},mode="idle"}[5m]) * 100)`,
  memUsedPct: (id: number) =>
    `100 - (windows_memory_physical_free_bytes{${mid(id)}} / windows_memory_physical_total_bytes{${mid(id)}} * 100)`,
  memAvail: (id: number) => `windows_memory_physical_free_bytes{${mid(id)}}`,
  diskRead: (id: number) =>
    `rate(windows_logical_disk_read_bytes_total{${mid(id)},volume!~"HarddiskVolume.*"}[5m])`,
  diskWrite: (id: number) =>
    `rate(windows_logical_disk_write_bytes_total{${mid(id)},volume!~"HarddiskVolume.*"}[5m])`,
  diskSpace: (id: number) =>
    `100 - (windows_logical_disk_free_bytes{${mid(
      id,
    )},volume=~"[A-Z]:.*"} / windows_logical_disk_size_bytes{${mid(id)},volume=~"[A-Z]:.*"} * 100)`,
  netIn: (id: number) => `rate(windows_net_bytes_received_total{${mid(id)},nic!~".*isatap.*"}[5m])`,
  netOut: (id: number) => `rate(windows_net_bytes_sent_total{${mid(id)},nic!~".*isatap.*"}[5m])`,
  uptime: (id: number) => `time() - windows_system_boot_time_timestamp{${mid(id)}}`,
};

// ── Formatters ────────────────────────────────────────────────────────────────

function fmtBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes.toFixed(0)} B`;
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
}

function fmtUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function fmtTime(ts: number) {
  return new Date(ts * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

// ── Data transformers ─────────────────────────────────────────────────────────

type ChartRow = Record<string, string | number>;

function singleSeries(results: PromRangeResult[]): ChartRow[] {
  const data = results[0]?.values ?? [];
  return data.map(([ts, val]) => ({ time: fmtTime(ts), value: parseFloat(val) }));
}

function multiSeries(results: PromRangeResult[], getKey: (m: Record<string, string>) => string): ChartRow[] {
  if (!results.length) return [];
  return (results[0].values ?? []).map(([ts], i) => {
    const row: ChartRow = { time: fmtTime(ts) };
    for (const r of results) {
      row[getKey(r.metric)] = parseFloat(r.values[i]?.[1] ?? "0");
    }
    return row;
  });
}

function mergedSeries(a: PromRangeResult[], aKey: string, b: PromRangeResult[], bKey: string): ChartRow[] {
  const base = a[0]?.values ?? b[0]?.values ?? [];
  return base.map(([ts], i) => ({
    time: fmtTime(ts),
    [aKey]: parseFloat(a[0]?.values[i]?.[1] ?? "0"),
    [bKey]: parseFloat(b[0]?.values[i]?.[1] ?? "0"),
  }));
}

function instantValue(results: PromInstantResult[]): number | null {
  const v = results[0]?.value?.[1];
  return v != null ? parseFloat(v) : null;
}

// ── Shared chart primitives ───────────────────────────────────────────────────

const TICK = { fill: "#8c909f", fontSize: 10 };

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function DarkTooltip(fmt: (v: number) => string) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return function TooltipContent({ active, payload, label }: any) {
    if (!active || !payload?.length) return null;
    return (
      <div className='rounded-md border border-rim bg-surface-highest px-3 py-2 text-xs shadow-xl min-w-[120px]'>
        <p className='text-ink-faint mb-1.5 font-mono'>{label}</p>
        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
        {payload.map((p: any) => (
          <div key={p.name} className='flex items-center gap-2 py-0.5'>
            <span className='size-1.5 rounded-full shrink-0' style={{ background: p.color }} />
            <span className='text-ink-dim'>{p.name}</span>
            <span className='text-ink font-mono ml-auto pl-3'>{fmt(p.value)}</span>
          </div>
        ))}
      </div>
    );
  };
}

function ChartCard({
  title,
  loading,
  empty,
  children,
}: {
  title: string;
  loading?: boolean;
  empty?: boolean;
  children: ReactNode;
}) {
  return (
    <div className='rounded-lg border border-rim bg-surface p-4'>
      <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mb-4'>{title}</p>
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

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className='rounded-lg border border-rim bg-surface px-4 py-3.5'>
      <p className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mb-1.5'>{label}</p>
      <p className='text-lg font-semibold text-ink font-mono leading-none'>{value}</p>
      {sub && <p className='text-[0.625rem] text-ink-faint mt-1'>{sub}</p>}
    </div>
  );
}

// ── Gradient defs ─────────────────────────────────────────────────────────────

function Gradients() {
  return (
    <defs>
      {[
        ["gCpu", C.blue],
        ["gMem", C.green],
        ["gDiskRead", C.amber],
        ["gDiskWrite", C.red],
        ["gNetIn", C.blue],
        ["gNetOut", C.purple],
      ].map(([id, color]) => (
        <linearGradient key={id} id={id} x1='0' y1='0' x2='0' y2='1'>
          <stop offset='5%' stopColor={color} stopOpacity={0.18} />
          <stop offset='95%' stopColor={color} stopOpacity={0} />
        </linearGradient>
      ))}
    </defs>
  );
}

const LEGEND_STYLE = { fontSize: 10, color: "#8c909f" };
const REFETCH_MS = 30_000;
const RANGE_SECS = 30 * 60;
const STEP = 60;

// ── Main component ────────────────────────────────────────────────────────────

export function MetricsTab({ machineId }: { machineId: number }) {
  if (!prometheusService.isConfigured()) {
    return (
      <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
        <Activity className='size-5 text-ink-faint mb-3' />
        <p className='text-sm font-semibold text-ink'>Metrics not configured</p>
      </div>
    );
  }

  if (!machineId) {
    return (
      <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
        <Activity className='size-5 text-ink-faint mb-3' />
        <p className='text-sm font-semibold text-ink'>Machine not connected</p>
      </div>
    );
  }

  const qOpts = { refetchInterval: REFETCH_MS, staleTime: 0 };

  // Instant (stat cards)
  const { data: iUptime } = useQuery({
    queryKey: ["pm-uptime", machineId],
    queryFn: () => prometheusService.instant(Q.uptime(machineId)),
    ...qOpts,
  });
  const { data: iMemAvail } = useQuery({
    queryKey: ["pm-memavail", machineId],
    queryFn: () => prometheusService.instant(Q.memAvail(machineId)),
    ...qOpts,
  });
  const { data: iMemPct } = useQuery({
    queryKey: ["pm-mempct", machineId],
    queryFn: () => prometheusService.instant(Q.memUsedPct(machineId)),
    ...qOpts,
  });
  const { data: iCpu } = useQuery({
    queryKey: ["pm-cpuinst", machineId],
    queryFn: () => prometheusService.instant(Q.cpuTotal(machineId)),
    ...qOpts,
  });

  // Range (charts) - queryFn computes time window fresh on each refetch
  const range = (key: string, qFn: () => Promise<PromRangeResult[]>) =>
    useQuery({ queryKey: ["pm-range", key, machineId], queryFn: qFn, ...qOpts });

  const { data: rCpu, isLoading: lCpu } = range("cpu", () => {
    const n = ts();
    return prometheusService.range(Q.cpuTotal(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: rCpuCore, isLoading: lCpuCore } = range("cpu-core", () => {
    const n = ts();
    return prometheusService.range(Q.cpuPerCore(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: rMem, isLoading: lMem } = range("mem", () => {
    const n = ts();
    return prometheusService.range(Q.memUsedPct(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: rDiskR, isLoading: lDisk } = range("disk-r", () => {
    const n = ts();
    return prometheusService.range(Q.diskRead(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: rDiskW } = range("disk-w", () => {
    const n = ts();
    return prometheusService.range(Q.diskWrite(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: rNetIn, isLoading: lNet } = range("net-in", () => {
    const n = ts();
    return prometheusService.range(Q.netIn(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: rNetOut } = range("net-out", () => {
    const n = ts();
    return prometheusService.range(Q.netOut(machineId), n - RANGE_SECS, n, STEP);
  });
  const { data: iDiskSpc, isLoading: lDiskSpc } = useQuery({
    queryKey: ["pm-disk-spc", machineId],
    queryFn: () => prometheusService.instant(Q.diskSpace(machineId)),
    ...qOpts,
  });

  // Derived stat values
  const cpuPct = instantValue(iCpu ?? []);
  const memPct = instantValue(iMemPct ?? []);
  const memAvailB = instantValue(iMemAvail ?? []);
  const uptimeSecs = instantValue(iUptime ?? []);

  // Chart data
  const cpuData = singleSeries(rCpu ?? []);
  const memData = singleSeries(rMem ?? []);
  const diskIoData = mergedSeries(rDiskR ?? [], "Read", rDiskW ?? [], "Write");
  const netData = mergedSeries(rNetIn ?? [], "In", rNetOut ?? [], "Out");
  const coreKeys = (rCpuCore ?? []).map((r) => `Core ${r.metric.core ?? r.metric.cpu ?? "?"}`);
  const coreData = multiSeries(rCpuCore ?? [], (m) => `Core ${m.core ?? m.cpu ?? "?"}`);
  const diskSpcData = (iDiskSpc ?? [])
    .map((r) => ({ volume: r.metric.volume ?? "?", used: parseFloat(r.value[1]) }))
    .sort((a, b) => a.volume.localeCompare(b.volume));

  const pctFmt = (v: number) => `${v.toFixed(1)}%`;
  const bytesFmt = (v: number) => `${fmtBytes(v)}/s`;

  return (
    <div className='space-y-4'>
      {/* Stat row */}
      <div className='grid grid-cols-4 gap-3'>
        <StatCard label='CPU Usage' value={cpuPct != null ? pctFmt(cpuPct) : "N/A"} />
        <StatCard
          label='Memory Used'
          value={memPct != null ? pctFmt(memPct) : "N/A"}
          sub={memAvailB != null ? `${fmtBytes(memAvailB)} available` : undefined}
        />
        <StatCard label='Uptime' value={uptimeSecs != null ? fmtUptime(uptimeSecs) : "N/A"} />
        <StatCard label='Refresh' value='30s' sub='auto-refreshing' />
      </div>

      {/* 2-column chart grid */}
      <div className='grid grid-cols-2 gap-4'>
        <ChartCard title='CPU Usage %' loading={lCpu} empty={!cpuData.length}>
          <ResponsiveContainer width='100%' height={160}>
            <AreaChart data={cpuData}>
              <Gradients />
              <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' vertical={false} />
              <XAxis
                dataKey='time'
                tick={TICK}
                axisLine={false}
                tickLine={false}
                interval='preserveStartEnd'
              />
              <YAxis
                tick={TICK}
                axisLine={false}
                tickLine={false}
                domain={[0, 100]}
                tickFormatter={(v) => `${v}%`}
                width={36}
              />
              <Tooltip content={DarkTooltip(pctFmt)} />
              <Area
                type='monotone'
                dataKey='value'
                name='CPU'
                stroke={C.blue}
                fill='url(#gCpu)'
                strokeWidth={1.5}
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title='Memory Used %' loading={lMem} empty={!memData.length}>
          <ResponsiveContainer width='100%' height={160}>
            <AreaChart data={memData}>
              <Gradients />
              <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' vertical={false} />
              <XAxis
                dataKey='time'
                tick={TICK}
                axisLine={false}
                tickLine={false}
                interval='preserveStartEnd'
              />
              <YAxis
                tick={TICK}
                axisLine={false}
                tickLine={false}
                domain={[0, 100]}
                tickFormatter={(v) => `${v}%`}
                width={36}
              />
              <Tooltip content={DarkTooltip(pctFmt)} />
              <Area
                type='monotone'
                dataKey='value'
                name='Memory'
                stroke={C.green}
                fill='url(#gMem)'
                strokeWidth={1.5}
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title='Disk I/O' loading={lDisk} empty={!diskIoData.length}>
          <ResponsiveContainer width='100%' height={160}>
            <AreaChart data={diskIoData}>
              <Gradients />
              <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' vertical={false} />
              <XAxis
                dataKey='time'
                tick={TICK}
                axisLine={false}
                tickLine={false}
                interval='preserveStartEnd'
              />
              <YAxis
                tick={TICK}
                axisLine={false}
                tickLine={false}
                tickFormatter={(v) => fmtBytes(v)}
                width={58}
              />
              <Tooltip content={DarkTooltip(bytesFmt)} />
              <Legend wrapperStyle={LEGEND_STYLE} />
              <Area
                type='monotone'
                dataKey='Read'
                stroke={C.amber}
                fill='url(#gDiskRead)'
                strokeWidth={1.5}
                dot={false}
              />
              <Area
                type='monotone'
                dataKey='Write'
                stroke={C.red}
                fill='url(#gDiskWrite)'
                strokeWidth={1.5}
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title='Network I/O' loading={lNet} empty={!netData.length}>
          <ResponsiveContainer width='100%' height={160}>
            <AreaChart data={netData}>
              <Gradients />
              <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' vertical={false} />
              <XAxis
                dataKey='time'
                tick={TICK}
                axisLine={false}
                tickLine={false}
                interval='preserveStartEnd'
              />
              <YAxis
                tick={TICK}
                axisLine={false}
                tickLine={false}
                tickFormatter={(v) => fmtBytes(v)}
                width={58}
              />
              <Tooltip content={DarkTooltip(bytesFmt)} />
              <Legend wrapperStyle={LEGEND_STYLE} />
              <Area
                type='monotone'
                dataKey='In'
                stroke={C.blue}
                fill='url(#gNetIn)'
                strokeWidth={1.5}
                dot={false}
              />
              <Area
                type='monotone'
                dataKey='Out'
                stroke={C.purple}
                fill='url(#gNetOut)'
                strokeWidth={1.5}
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </ChartCard>
      </div>

      {/* Disk space - full width bar */}
      <ChartCard title='Disk Space Used % Per Volume' loading={lDiskSpc} empty={!diskSpcData.length}>
        <ResponsiveContainer width='100%' height={Math.max(80, diskSpcData.length * 36)}>
          <BarChart data={diskSpcData} layout='vertical' margin={{ left: 0, right: 16 }}>
            <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' horizontal={false} />
            <XAxis
              type='number'
              tick={TICK}
              axisLine={false}
              tickLine={false}
              domain={[0, 100]}
              tickFormatter={(v) => `${v}%`}
            />
            <YAxis
              type='category'
              dataKey='volume'
              tick={TICK}
              axisLine={false}
              tickLine={false}
              width={32}
            />
            <Tooltip content={DarkTooltip(pctFmt)} />
            <Bar dataKey='used' name='Used' fill={C.blue} radius={[0, 3, 3, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </ChartCard>

      {/* CPU per core - only when multi-core */}
      {coreKeys.length > 1 && (
        <ChartCard title='CPU Per Core %' loading={lCpuCore} empty={!coreData.length}>
          <ResponsiveContainer width='100%' height={160}>
            <LineChart data={coreData}>
              <CartesianGrid stroke='#2a2a2a' strokeDasharray='3 3' vertical={false} />
              <XAxis
                dataKey='time'
                tick={TICK}
                axisLine={false}
                tickLine={false}
                interval='preserveStartEnd'
              />
              <YAxis
                tick={TICK}
                axisLine={false}
                tickLine={false}
                domain={[0, 100]}
                tickFormatter={(v) => `${v}%`}
                width={36}
              />
              <Tooltip content={DarkTooltip(pctFmt)} />
              <Legend wrapperStyle={LEGEND_STYLE} />
              {coreKeys.map((key, i) => (
                <Line
                  key={key}
                  type='monotone'
                  dataKey={key}
                  stroke={MULTI_PALETTE[i % MULTI_PALETTE.length]}
                  strokeWidth={1}
                  dot={false}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </ChartCard>
      )}
    </div>
  );
}

function ts() {
  return Math.floor(Date.now() / 1000);
}
