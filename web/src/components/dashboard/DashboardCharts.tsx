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

// Recharts-dependent Dashboard visuals, split into their own lazily-loaded chunk (see
// usage in pages/Dashboard.tsx) so the rest of the dashboard (header, stat cards) can
// paint immediately instead of waiting on this library to download.

function fmtPct(v: number) {
  return `${v.toFixed(1)}%`;
}

export function RadialGauge({ label, value }: { label: string; value: number | null }) {
  const pct = Math.min(Math.max(value ?? 0, 0), 100);
  const color = pct >= 80 ? "#f85149" : pct >= 60 ? "#d29922" : "#3fb950";
  const data = [{ v: pct }, { v: 100 - pct }];

  return (
    <div className='flex flex-col items-center'>
      <div className='relative w-full' style={{ height: 162 }}>
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
          <span className='text-[22px] font-semibold font-mono text-ink leading-none'>
            {value != null ? fmtPct(value) : "N/A"}
          </span>
        </div>
      </div>
      <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>{label}</span>
    </div>
  );
}

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

export function HostCpuLineChart({
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
    <ResponsiveContainer width='100%' height={234}>
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
