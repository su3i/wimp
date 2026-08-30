import { useEffect, useState } from "react";
import { AlertTriangle, CheckCircle2, CircleDot, ShieldAlert } from "lucide-react";
import type { Incident } from "@/types";
import { levelConfig, splitTitle } from "@/utils/notifications";
import { absoluteTime, timeAgo } from "@/utils/time";
import { cn } from "@/utils/cn";

// Incidents render as a commit graph rather than as table rows. An incident is a span, not
// an event, and a span has a shape: it starts, it runs for some time, it ends. Two rows in
// a flat feed force the reader to pair a failure with its recovery by eye and subtract the
// timestamps; a rail that visibly connects the two carries the same information for free.

// Column the rail runs down, and the size of the node sitting on it. Kept as constants so
// the rail segments, the nodes and the row indent cannot drift out of alignment.
const RAIL_X = 22;
const NODE = 11;

// A clock, rather than a timestamp sampled once while rendering. Open incidents show a
// running duration, so the value has to advance on its own; reading Date.now() during
// render would both be impure and freeze the label the moment nothing else re-rendered.
function useNow(intervalMs: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}

function fmtDuration(ms: number): string {
  const totalMinutes = Math.max(0, Math.round(ms / 60_000));
  if (totalMinutes < 1) return "under a minute";
  if (totalMinutes < 60) return `${totalMinutes}m`;
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours < 24) return minutes ? `${hours}h ${minutes}m` : `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

function incidentDuration(incident: Incident, now: number): string {
  const start = new Date(incident.StartedAt).getTime();
  const end = incident.ResolvedAt ? new Date(incident.ResolvedAt).getTime() : now;
  return fmtDuration(end - start);
}

function LevelBadge({ level }: { level: string }) {
  const cfg = levelConfig(level);
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[0.625rem] font-semibold uppercase tracking-wide",
        cfg.cls,
      )}
    >
      {cfg.label}
    </span>
  );
}

function IncidentNode({ incident, isLast, now }: { incident: Incident; isLast: boolean; now: number }) {
  const resolved = incident.Status === "resolved";
  const { event: openedEvent } = splitTitle(incident.OpenedTitle);
  const { event: resolvedEvent } = splitTitle(incident.ResolvedTitle);

  return (
    <div className='relative'>
      {/* The rail. It runs the full height of an incident that has both ends, and stops at
          the open node for one still running - an unfinished branch should look unfinished.
          The last incident on the page never draws past its final node. */}
      <span
        className='absolute top-0 w-px bg-rim'
        style={{ left: RAIL_X, height: isLast && !resolved ? "1.25rem" : "100%" }}
      />

      {/* Opening event. Filled node, because the incident began here. */}
      <div className='relative flex items-start gap-3 py-2.5 pr-4' style={{ paddingLeft: RAIL_X + 18 }}>
        <span
          className={cn(
            "absolute z-10 rounded-full border-2 bg-canvas",
            resolved ? "border-ink-faint" : "border-danger bg-danger",
          )}
          style={{ left: RAIL_X - NODE / 2, width: NODE, height: NODE, top: 12 }}
        />
        <div className='min-w-0 flex-1'>
          <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
            <LevelBadge level={incident.Level} />
            <span className='text-xs font-medium text-ink'>{openedEvent || incident.Kind}</span>
            <span className='font-mono text-[0.6875rem] text-ink-dim'>{incident.Instance}</span>
            {incident.Subject && !incident.Subject.startsWith("application:") && (
              <span className='rounded border border-rim bg-surface-high px-1.5 py-0.5 font-mono text-[0.625rem] text-ink-faint'>
                {incident.Subject}
              </span>
            )}
          </div>
          {incident.OpenedDetail && (
            <p className='mt-1 text-[0.6875rem] text-ink-faint'>{incident.OpenedDetail}</p>
          )}
        </div>
        <div className='shrink-0 text-right'>
          <p className='text-[0.6875rem] text-ink-dim tabular-nums whitespace-nowrap'>
            {timeAgo(incident.StartedAt)}
          </p>
          <p className='text-[0.625rem] text-ink-faint/70 tabular-nums whitespace-nowrap'>
            {absoluteTime(incident.StartedAt)}
          </p>
        </div>
      </div>

      {/* Closing event, or the still-running state */}
      {resolved ? (
        <div className='relative flex items-start gap-3 pb-5 pr-4' style={{ paddingLeft: RAIL_X + 18 }}>
          <span
            className='absolute z-10 rounded-full border-2 border-success bg-canvas'
            style={{ left: RAIL_X - NODE / 2, width: NODE, height: NODE, top: 1 }}
          />
          <div className='min-w-0 flex-1'>
            <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
              <CheckCircle2 className='size-3 shrink-0 text-success' />
              <span className='text-xs text-ink-dim'>{resolvedEvent || "Recovered"}</span>
              <span className='text-[0.625rem] text-ink-faint'>
                after {incidentDuration(incident, now)}
              </span>
            </div>
            {incident.ResolvedDetail && (
              <p className='mt-1 text-[0.6875rem] text-ink-faint'>{incident.ResolvedDetail}</p>
            )}
          </div>
          <div className='shrink-0 text-right'>
            <p className='text-[0.6875rem] text-ink-dim tabular-nums whitespace-nowrap'>
              {timeAgo(incident.ResolvedAt)}
            </p>
            <p className='text-[0.625rem] text-ink-faint/70 tabular-nums whitespace-nowrap'>
              {absoluteTime(incident.ResolvedAt)}
            </p>
          </div>
        </div>
      ) : (
        <div className='relative flex items-center gap-2 pb-5 pr-4' style={{ paddingLeft: RAIL_X + 18 }}>
          <CircleDot className='size-3 shrink-0 animate-pulse text-danger' />
          <span className='text-[0.6875rem] font-medium text-danger'>
            Ongoing for {incidentDuration(incident, now)}
          </span>
        </div>
      )}
    </div>
  );
}

export function IncidentTimeline({
  incidents,
  isLoading,
  isError,
}: {
  incidents: Incident[];
  isLoading: boolean;
  isError: boolean;
}) {
  // One clock for the whole list, so every duration on screen is measured from the same
  // instant. Minute resolution is all fmtDuration renders, so ticking faster would just be
  // re-rendering the same text.
  const now = useNow(30_000);

  return (
    <div className='rounded-lg border border-rim bg-surface overflow-hidden'>
      <div className='min-h-[400px]'>
        {isLoading ? (
          <div className='space-y-1 p-4'>
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className='flex items-center gap-3 py-3 animate-pulse'>
                <div className='size-2.5 shrink-0 rounded-full bg-surface-high' />
                <div className='h-2.5 w-48 rounded bg-surface-high' />
                <div className='ml-auto h-2.5 w-16 rounded bg-surface-high' />
              </div>
            ))}
          </div>
        ) : isError ? (
          <div className='flex h-[400px] items-center justify-center gap-2 text-xs text-danger'>
            <AlertTriangle className='size-3.5' /> Failed to load incidents.
          </div>
        ) : incidents.length === 0 ? (
          <div className='flex h-[400px] flex-col items-center justify-center gap-2 text-ink-faint'>
            <ShieldAlert className='size-5 opacity-30' />
            <p className='text-xs'>No incidents recorded.</p>
            <p className='max-w-xs text-center text-[0.625rem]'>
              An incident opens when something fails and closes when it recovers.
            </p>
          </div>
        ) : (
          <div className='py-2'>
            {incidents.map((incident, i) => (
              <IncidentNode
                key={incident.ID}
                incident={incident}
                isLast={i === incidents.length - 1}
                now={now}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
