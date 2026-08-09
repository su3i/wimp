import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";

// Recharts-dependent Dashboard visuals, split into their own lazily-loaded chunk (see
// usage in pages/Dashboard.tsx) so the rest of the dashboard (header, stat cards) can
// paint immediately instead of waiting on this library to download.

const TICK_STYLE = { fill: "#8b949e", fontSize: 10 };
export const LINE_COLORS = ["#3b82f6", "#22c55e", "#f59e0b", "#ef4444", "#a855f7", "#06b6d4", "#f97316", "#ec4899"];
// Matches the height passed to ChartSkeleton (the Suspense fallback in pages/Dashboard.tsx)
// - the "no data yet" state below needs to be the same height as a loaded chart too, or
// the chart's box visibly grows the moment data arrives.
const CHART_HEIGHT = 234;

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

export function HostLineChart({
  rows,
  keys,
  colors,
}: {
  rows: Record<string, string | number>[];
  keys: string[];
  // Maps a key (hostname) to a fixed color, so the same machine gets the same line
  // color on every chart it appears in, not just a per-chart positional color. Falls
  // back to position-based color for any key not in the map.
  colors?: Record<string, string>;
}) {
  if (!rows.length || !keys.length) {
    return (
      <div className='flex items-center justify-center' style={{ height: CHART_HEIGHT }}>
        <p className='text-xs text-ink-faint'>No data</p>
      </div>
    );
  }

  return (
    <ResponsiveContainer width='100%' height={CHART_HEIGHT}>
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
            stroke={colors?.[key] ?? LINE_COLORS[i % LINE_COLORS.length]}
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
