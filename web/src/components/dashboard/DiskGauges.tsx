import { useCallback, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { prometheusService } from "@/services/prometheus.service";
import { InfoTooltip } from "@/components/ui/InfoTooltip";
import { cn } from "@/utils/cn";

// Per-host disk usage, one dial per connected host. Deliberately not a chart: disk is the
// one host metric where the current value is the whole story - nobody needs the shape of
// the last hour to know that a volume is at 96%.

// Width of one gauge cell and the gap between cells. Paging maths below derives how many
// fit from these, so they have to stay in sync with the rendered cell.
const CELL_WIDTH = 104;
const CELL_GAP = 16;
// Two dials per page. The card shares a row with the aggregate stat tiles and there isn't
// width for a third without shrinking the dials past the point where the hostnames stay
// comfortable. Still a ceiling rather than a fixed count, so a genuinely narrow viewport
// falls back to one instead of overflowing.
const MAX_PER_PAGE = 2;

const GAUGE_SIZE = 58;
const GAUGE_STROKE = 6;
const RADIUS = (GAUGE_SIZE - GAUGE_STROKE) / 2;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

// Thresholds mirror the low-disk alert's intent (fires below 5% free, i.e. above 95%
// used), with an amber band ahead of it so the dial turns before the alert does.
function usageColor(pct: number) {
  if (pct >= 95) return { stroke: "var(--danger)", text: "text-danger" };
  if (pct >= 85) return { stroke: "var(--warning)", text: "text-warning" };
  return { stroke: "var(--success)", text: "text-success" };
}

export interface HostDisk {
  machineId: number;
  hostname: string;
  usedPct: number;
  volume: string;
}

function Gauge({ host }: { host: HostDisk }) {
  const pct = Math.max(0, Math.min(100, host.usedPct));
  const color = usageColor(pct);
  // strokeDashoffset counts backwards from a full circle, so a 0% dial is fully offset
  // (invisible) and a 100% dial has no offset at all.
  const offset = CIRCUMFERENCE * (1 - pct / 100);

  return (
    <div
      className='flex shrink-0 flex-col items-center gap-1.5'
      style={{ width: CELL_WIDTH }}
      title={`${host.hostname} - volume ${host.volume} is ${pct.toFixed(1)}% full`}
    >
      <div className='relative' style={{ width: GAUGE_SIZE, height: GAUGE_SIZE }}>
        <svg
          width={GAUGE_SIZE}
          height={GAUGE_SIZE}
          // Rotated so the arc starts at 12 o'clock instead of 3 o'clock.
          className='-rotate-90'
        >
          <circle
            cx={GAUGE_SIZE / 2}
            cy={GAUGE_SIZE / 2}
            r={RADIUS}
            fill='none'
            stroke='var(--rim)'
            strokeWidth={GAUGE_STROKE}
          />
          <circle
            cx={GAUGE_SIZE / 2}
            cy={GAUGE_SIZE / 2}
            r={RADIUS}
            fill='none'
            stroke={color.stroke}
            strokeWidth={GAUGE_STROKE}
            strokeLinecap='round'
            strokeDasharray={CIRCUMFERENCE}
            strokeDashoffset={offset}
            className='transition-[stroke-dashoffset] duration-500'
          />
        </svg>
        <div className='absolute inset-0 flex items-center justify-center'>
          <span className={cn("font-mono text-xs font-semibold", color.text)}>{Math.round(pct)}%</span>
        </div>
      </div>
      {/* Hostname and volume share one line rather than stacking - the card sits in a row
          of short stat tiles, and a second text line was the difference between it fitting
          that row and dictating its height. */}
      <div className='flex w-full items-baseline justify-center gap-1 px-0.5'>
        <span className='min-w-0 truncate font-mono text-[0.6875rem] text-ink-dim'>
          {host.hostname}
        </span>
        <span className='shrink-0 text-[0.625rem] text-ink-faint'>{host.volume}</span>
      </div>
    </div>
  );
}

function PagerButton({
  direction,
  disabled,
  onClick,
}: {
  direction: "prev" | "next";
  disabled: boolean;
  onClick: () => void;
}) {
  const Icon = direction === "prev" ? ChevronLeft : ChevronRight;
  return (
    <button
      type='button'
      onClick={onClick}
      disabled={disabled}
      aria-label={direction === "prev" ? "Previous hosts" : "Next hosts"}
      className={cn(
        "flex size-6 shrink-0 items-center justify-center rounded-md border border-rim transition-colors",
        disabled
          ? "cursor-not-allowed text-ink-faint/40"
          : "cursor-pointer text-ink-faint hover:bg-surface-high hover:text-ink",
      )}
    >
      <Icon className='size-3.5' />
    </button>
  );
}

export function DiskGauges({
  machineIds,
  hostNames,
}: {
  machineIds: number[];
  hostNames: Map<number, string>;
}) {
  // 0 means "not measured yet" - rendering nothing for one frame beats guessing a page
  // size and reflowing the moment the real width arrives.
  const [perPage, setPerPage] = useState(0);
  const [page, setPage] = useState(0);
  const observerRef = useRef<ResizeObserver | null>(null);

  // A callback ref rather than a mount effect, because this component renders null until
  // the host list has loaded. A [] -dependency effect would run once against a ref that is
  // still null, bail out, and never get another chance once the card actually mounted -
  // leaving the page size at 0 and the card permanently empty. This runs whenever the node
  // attaches or detaches, however late that is.
  //
  // The card's width comes from its grid column, not from its contents, so measuring the
  // track cannot feed back into what the track is asked to hold.
  const measureRef = useCallback((node: HTMLDivElement | null) => {
    observerRef.current?.disconnect();
    observerRef.current = null;
    if (!node) return;

    const observer = new ResizeObserver(() => {
      const width = node.clientWidth;
      if (width <= 0) return;
      const fits = Math.floor((width + CELL_GAP) / (CELL_WIDTH + CELL_GAP));
      setPerPage(Math.min(MAX_PER_PAGE, Math.max(1, fits)));
    });
    observer.observe(node);
    observerRef.current = observer;
  }, []);

  const idKey = machineIds.slice().sort((a, b) => a - b).join("|");
  const enabled = prometheusService.isConfigured() && machineIds.length > 0;

  const { data, isLoading } = useQuery({
    queryKey: ["d-disk-usage", idKey],
    enabled,
    refetchInterval: 60_000,
    // Per volume rather than pre-aggregated per host, so the worst volume can be named
    // rather than just scored - "C: at 96%" is actionable in a way that "96%" is not.
    queryFn: () =>
      prometheusService.instant(
        `100 - (windows_logical_disk_free_bytes{machine_id=~"${idKey}",volume=~"[A-Z]:.*"} / windows_logical_disk_size_bytes{machine_id=~"${idKey}",volume=~"[A-Z]:.*"} * 100)`,
      ),
  });

  const hosts = useMemo<HostDisk[]>(() => {
    const worstByMachine = new Map<number, HostDisk>();
    for (const r of data ?? []) {
      const machineId = Number(r.metric.machine_id);
      const usedPct = Number(r.value[1]);
      if (Number.isNaN(machineId) || !isFinite(usedPct)) continue;

      const existing = worstByMachine.get(machineId);
      if (existing && existing.usedPct >= usedPct) continue;
      worstByMachine.set(machineId, {
        machineId,
        hostname: hostNames.get(machineId) ?? String(machineId),
        usedPct,
        volume: r.metric.volume ?? "?",
      });
    }
    // Fullest host first: the dial you need to see is never the one you have to scroll to.
    return [...worstByMachine.values()].sort((a, b) => b.usedPct - a.usedPct);
  }, [data, hostNames]);

  const totalPages = perPage > 0 ? Math.max(1, Math.ceil(hosts.length / perPage)) : 1;
  // Derived, not stored: the window can be resized to fit every host at once while the
  // user sits on page 3, and a stored page index would strand them past the end.
  const currentPage = Math.min(page, totalPages - 1);
  const visible = perPage > 0 ? hosts.slice(currentPage * perPage, currentPage * perPage + perPage) : [];
  const paged = totalPages > 1;

  if (!enabled) return null;

  return (
    // Padding matches the aggregate stat tiles it now shares a row with, so the four
    // cards read as one set rather than three plus a visitor.
    <div className='rounded-lg border border-rim bg-surface px-[18px] py-[18px] flex flex-col gap-2.5 min-h-[124px]'>
      <div className='flex items-center justify-between gap-3'>
        <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
          Disk Usage
          <InfoTooltip text="Each host's disk usage." />
        </span>
        {paged && (
          <div className='flex items-center gap-1.5'>
            <span className='text-[0.625rem] text-ink-faint tabular-nums'>
              {currentPage + 1}/{totalPages}
            </span>
            <PagerButton
              direction='prev'
              disabled={currentPage === 0}
              onClick={() => setPage(Math.max(0, currentPage - 1))}
            />
            <PagerButton
              direction='next'
              disabled={currentPage >= totalPages - 1}
              onClick={() => setPage(Math.min(totalPages - 1, currentPage + 1))}
            />
          </div>
        )}
      </div>

      <div ref={measureRef} className='flex flex-1 items-center overflow-hidden'>
        {isLoading ? (
          <div className='flex w-full items-center justify-center'>
            <div className='size-4 animate-spin rounded-full border-2 border-primary border-t-transparent' />
          </div>
        ) : hosts.length === 0 ? (
          <span className='w-full text-center text-xs text-ink-faint'>No disk data</span>
        ) : (
          <div className='flex w-full justify-center' style={{ gap: CELL_GAP }}>
            {visible.map((host) => (
              <Gauge key={host.machineId} host={host} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
