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
const GRID_STROKE = "#30363d";
// One width for every chart, whatever it plots. Recharts reserves this whole strip for the
// axis and right-aligns the labels against the plot area, so a chart with short labels and
// a wide axis shows the surplus as an empty gutter down its left edge. Sizing it per format
// also left the three charts in a row with plot areas that didn't line up with each other.
// 36px clears the widest label either format produces ("100%" and "12.5k").
const Y_AXIS_WIDTH = 36;
export const LINE_COLORS = ["#3b82f6", "#22c55e", "#f59e0b", "#ef4444", "#a855f7", "#06b6d4", "#f97316", "#ec4899"];
// Matches the height passed to ChartSkeleton (the Suspense fallback in pages/Dashboard.tsx)
// - the "no data yet" state below needs to be the same height as a loaded chart too, or
// the chart's box visibly grows the moment data arrives.
const CHART_HEIGHT = 234;

// Percent series are pinned to a 0-100 axis; rate series (requests/sec) have no natural
// ceiling, so recharts picks the domain from the data instead.
export type ChartFormat = "percent" | "rate";

function formatValue(v: number, format: ChartFormat) {
  if (format === "percent") return `${v.toFixed(1)}%`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k/s`;
  return `${v.toFixed(1)}/s`;
}

function formatAxis(v: number, format: ChartFormat) {
  if (format === "percent") return `${v}%`;
  if (v >= 1000) return `${v / 1000}k`;
  return String(v);
}

function TooltipContentFor(format: ChartFormat) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return function HostTooltipContent({ active, payload, label }: any) {
    if (!active || !payload?.length) return null;

    // Only rate series sum to anything meaningful - adding up per-host percentages would
    // produce a number that looks authoritative and means nothing. With a single host the
    // total is just that host's line repeated, so it's suppressed there too.
    const showTotal = format === "rate" && payload.length > 1;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const total = payload.reduce((sum: number, p: any) => sum + (Number(p.value) || 0), 0);

    return (
      <div className='rounded-md border border-rim bg-surface-highest px-3 py-2 text-xs shadow-xl min-w-[150px]'>
        <p className='font-mono text-ink-faint mb-1.5'>{label}</p>
        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
        {payload.map((p: any) => (
          <div key={p.name} className='flex items-center gap-2 py-0.5'>
            <span className='size-1.5 rounded-full shrink-0' style={{ background: p.color }} />
            <span className='text-ink-dim flex-1 truncate'>{p.name}</span>
            <span className='font-mono font-semibold text-ink ml-2'>
              {formatValue(p.value as number, format)}
            </span>
          </div>
        ))}
        {showTotal && (
          <div className='mt-1.5 flex items-center gap-2 border-t border-rim pt-1.5'>
            <span className='size-1.5 shrink-0' />
            <span className='flex-1 font-medium text-ink-dim'>Total</span>
            <span className='font-mono font-semibold text-ink ml-2'>
              {formatValue(total, format)}
            </span>
          </div>
        )}
      </div>
    );
  };
}

export function HostLineChart({
  rows,
  keys,
  colors,
  format = "percent",
}: {
  rows: Record<string, string | number>[];
  keys: string[];
  // Maps a key (hostname) to a fixed color, so the same machine gets the same line
  // color on every chart it appears in, not just a per-chart positional color. Falls
  // back to position-based color for any key not in the map.
  colors?: Record<string, string>;
  format?: ChartFormat;
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
        <CartesianGrid stroke={GRID_STROKE} strokeDasharray='3 3' vertical={false} />
        <XAxis
          dataKey='time'
          tick={TICK_STYLE}
          axisLine={false}
          tickLine={false}
          interval='preserveStartEnd'
          minTickGap={24}
        />
        <YAxis
          domain={format === "percent" ? [0, 100] : [0, "auto"]}
          tick={TICK_STYLE}
          // The x-axis reads as having a baseline because the lowest horizontal gridline
          // sits on it; the y-axis had nothing closing off the left edge, so it gets an
          // explicit axis line in the same colour as the grid.
          axisLine={{ stroke: GRID_STROKE }}
          tickLine={false}
          tickFormatter={(v) => formatAxis(v as number, format)}
          width={Y_AXIS_WIDTH}
        />
        <Tooltip content={TooltipContentFor(format)} />
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
