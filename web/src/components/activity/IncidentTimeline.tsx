import { useEffect, useState } from "react";
import { AlertTriangle, Check, CheckCircle2, ChevronRight, CircleAlert, CircleDot, Clock, ExternalLink, Loader2 } from "lucide-react";
import type { Incident } from "@/types";
import { levelConfig, splitTitle } from "@/utils/notifications";
import { absoluteTime, timeAgo } from "@/utils/time";
import { cn } from "@/utils/cn";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";

// A nested tree, two rails deep. The outer spine belongs to the day and runs from its
// heading down to the last entry under it. Each incident branches off that spine, and its
// own inner rail links the two events it is made of - the outcome on top, the trigger that
// caused it underneath.
//
//   | Incidents on Aug 30, 2026
//   |
//   |--|- Recovered                  2h ago
//   |  |
//   |--|- CRITICAL  High CPU         4h ago
//   |
//   |--|- Recovered                  6h ago
//   |  |
//   |--|- CRITICAL  High CPU         7h ago
//
// Every offset below is measured from these constants, so the branch meets the spine and
// the node at exactly one height and nothing needs nudging by eye.
const SPINE_X = 9;
const NODE_X = 31;
const CONTENT_X = 46;
// The day heading sits a little right of the entries beneath it, so the branches read as
// hanging off it rather than starting level with it. One constant if it wants nudging.
const HEADER_X = 2;
// Rails are 2px and drawn in the stronger rim tone: at 1px the tree read as a hairline
// behind the content rather than as the structure holding it.
const LINE_W = 2;
// How far the day's spine runs past the first and last branch on it. Ending flush with the
// outermost nodes made the rail look cropped; a little overshoot reads as a stem the
// branches hang from.
const OVERSHOOT = -8;
// Node and lead height are chosen together: the node is centred inside the height of an
// entry's first line, which is what puts it level with the text beside it rather than a few
// pixels below. Every value here is an integer so nothing lands on a half pixel.
const NODE = 10;
const LEAD = 20;
const NODE_TOP = (LEAD - NODE) / 2;
const NODE_CENTER = LEAD / 2;
// The branch stops at the node's edge rather than running underneath it.
const BRANCH_END = NODE_X - NODE / 2;
// How far the branch bows as it leaves the spine. Large enough to read as a curve at this
// scale, small enough that it has straightened out before reaching the node - the curly
// brace feel comes from the bow, not from a quarter circle.
const BEND = 8;
// The curve needs room above the node's centre line to leave the spine in, plus a little
// slack below so the stroke cap is not clipped.
const BRANCH_H = NODE_CENTER + LINE_W;

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

interface Group {
  key: string;
  label: string;
  incidents: Incident[];
}

// Groups the feed the way a commit history groups by day, with one exception: anything
// still running is collected at the top under its own heading rather than filed under the
// date it started. An ongoing incident is current state, not history, and burying a
// four-day-old outage under "Aug 26" would read as though it were over.
//
// The server already returns open first, then newest first, so appending in order is
// enough to keep both the groups and their contents correctly sorted.
function groupByDate(incidents: Incident[]): Group[] {
  const groups: Group[] = [];
  const byDate = new Map<string, Group>();
  let ongoing: Group | null = null;

  for (const incident of incidents) {
    if (incident.Status === "open") {
      if (!ongoing) {
        ongoing = { key: "ongoing", label: "Ongoing", incidents: [] };
        groups.push(ongoing);
      }
      ongoing.incidents.push(incident);
      continue;
    }

    const started = new Date(incident.StartedAt);
    const key = started.toDateString();
    let group = byDate.get(key);
    if (!group) {
      group = {
        key,
        label: `Incidents on ${started.toLocaleDateString(undefined, {
          month: "short",
          day: "numeric",
          year: "numeric",
        })}`,
        incidents: [],
      };
      byDate.set(key, group);
      groups.push(group);
    }
    group.incidents.push(incident);
  }

  return groups;
}

// Severity as a colour, applied to an incident's nodes. The rails stay neutral: they are
// structure, and colouring them made the whole tree read as an alarm rather than as the
// frame the alarms hang on.
//
// Sev and critical share the danger tone deliberately - there is no fifth hue that reads
// as "worse than red", and the badge already names which of the two it is.
const LEVEL_TONE: Record<string, { border: string; text: string }> = {
  sev: { border: "border-danger", text: "text-danger" },
  critical: { border: "border-danger", text: "text-danger" },
  warning: { border: "border-warning", text: "text-warning" },
  info: { border: "border-ink-dim", text: "text-ink-dim" },
};

function levelTone(level: string) {
  return LEVEL_TONE[level] ?? LEVEL_TONE.info;
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

function Timestamp({ at }: { at: string | null }) {
  if (!at) return null;
  return (
    <div className='shrink-0 text-right'>
      <p className='text-[0.6875rem] text-ink-dim tabular-nums whitespace-nowrap'>{timeAgo(at)}</p>
      <p className='text-[0.625rem] text-ink-faint/70 tabular-nums whitespace-nowrap'>
        {absoluteTime(at)}
      </p>
    </div>
  );
}

// Fixed-height first line, so whatever it holds - a tall badge, a short icon, bare text -
// the node beside it stays centred on the same line.
function Lead({ children }: { children: React.ReactNode }) {
  return (
    <div className='flex flex-wrap items-center gap-x-2 gap-y-1' style={{ minHeight: LEAD }}>
      {children}
    </div>
  );
}

// One event: its slice of the day's spine, the branch out to it, its slice of the
// incident's inner rail, and the node where the two meet.
function EventRow({
  tone,
  filled,
  spineToBottom,
  innerAbove,
  innerBelow,
  className,
  children,
}: {
  // Severity colour, applied to the node only.
  tone: string;
  filled?: boolean;
  // False only on the last event of a day, where the spine has nothing further to reach.
  spineToBottom: boolean;
  innerAbove: boolean;
  innerBelow: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={cn("relative", className)} style={{ paddingLeft: CONTENT_X }}>
      {/* The day's spine */}
      <span
        className='absolute bg-rim-strong'
        style={
          spineToBottom
            ? { left: SPINE_X - LINE_W / 2, width: LINE_W, top: 0, bottom: 0 }
            : { left: SPINE_X - LINE_W / 2, width: LINE_W, top: 0, height: NODE_CENTER + OVERSHOOT }
        }
      />
      {/* Branch: bows out of the spine and straightens into the node */}
      <svg
        className='pointer-events-none absolute'
        style={{ left: 0, top: 0, width: NODE_X, height: BRANCH_H }}
        aria-hidden
      >
        <path
          d={`M ${SPINE_X} ${NODE_CENTER - BEND} Q ${SPINE_X} ${NODE_CENTER} ${SPINE_X + BEND} ${NODE_CENTER} L ${BRANCH_END} ${NODE_CENTER}`}
          fill='none'
          stroke='var(--rim-strong)'
          strokeWidth={LINE_W}
          strokeLinecap='round'
        />
      </svg>
      {/* The incident's own rail, linking its two events */}
      {innerAbove && (
        <span
          className='absolute bg-rim-strong'
          style={{ left: NODE_X - LINE_W / 2, width: LINE_W, top: 0, height: NODE_CENTER }}
        />
      )}
      {innerBelow && (
        <span
          className='absolute bg-rim-strong'
          style={{ left: NODE_X - LINE_W / 2, width: LINE_W, top: NODE_CENTER, bottom: 0 }}
        />
      )}
      <span
        className={cn("absolute rounded-full border-2 bg-canvas", tone, filled && "bg-current")}
        style={{ left: NODE_X - NODE / 2, top: NODE_TOP, width: NODE, height: NODE }}
      />
      {children}
    </div>
  );
}

function IncidentEntry({
  incident,
  now,
  onResolve,
  onOpenDetail,
  resolving,
  isLastInGroup,
}: {
  incident: Incident;
  now: number;
  onResolve: (incident: Incident) => void;
  onOpenDetail: (incident: Incident) => void;
  resolving: boolean;
  isLastInGroup: boolean;
}) {
  // Collapsed by default. The detail is the measurement behind the alert, which is worth
  // having but not worth reading on every row while scanning a day.
  const [showDetail, setShowDetail] = useState(false);
  const tone = levelTone(incident.Level);
  const hasDetail = !!incident.OpenedDetail;
  const resolved = incident.Status === "resolved";
  const { event: openedEvent } = splitTitle(incident.OpenedTitle);
  const { event: resolvedEvent } = splitTitle(incident.ResolvedTitle);

  return (
    <>
      {/* ── Outcome: the newer of the two events ─────────────────────────── */}
      <EventRow
        tone={resolved ? "border-success text-success" : cn(tone.border, tone.text)}
        filled={!resolved}
        spineToBottom
        innerAbove={false}
        innerBelow
      >
        <div className='flex min-w-0 items-start gap-3 pb-4'>
          <div className='min-w-0 flex-1'>
            {resolved ? (
              <Lead>
                <CheckCircle2 className='size-3 shrink-0 text-success' />
                <span className='text-xs text-ink-dim'>{resolvedEvent || "Recovered"}</span>
                <span className='text-[0.625rem] text-ink-faint'>
                  after {incidentDuration(incident, now)}
                </span>
              </Lead>
            ) : (
              <Lead>
                <CircleDot className={cn("size-3 shrink-0 animate-pulse", tone.text)} />
                <span className={cn("text-xs font-medium", tone.text)}>
                  Ongoing for {incidentDuration(incident, now)}
                </span>
                {/* Not every condition reports its own recovery - a decommissioned host or
                    a silenced recovery alert leaves an incident with nothing to close it. */}
                <button
                  type='button'
                  disabled={resolving}
                  onClick={() => onResolve(incident)}
                  className={cn(
                    "flex shrink-0 items-center gap-1.5 rounded-md border border-rim px-2 py-0.5 text-[0.625rem] font-medium transition-colors",
                    resolving
                      ? "cursor-not-allowed text-ink-faint"
                      : "cursor-pointer text-ink-faint hover:border-success/40 hover:bg-surface-high hover:text-success",
                  )}
                >
                  {resolving ? <Loader2 className='size-3 animate-spin' /> : <Check className='size-3' />}
                  Mark as resolved
                </button>
              </Lead>
            )}
          </div>
          <Timestamp at={resolved ? incident.ResolvedAt : null} />
        </div>
      </EventRow>

      {/* ── Trigger: what started it ─────────────────────────────────────── */}
      <EventRow
        tone={tone.border}
        filled={!resolved}
        spineToBottom={!isLastInGroup}
        innerAbove
        innerBelow={false}
        className={isLastInGroup ? undefined : "pb-5"}
      >
        <div className='flex min-w-0 items-start gap-3'>
          <div className='min-w-0 flex-1'>
            <Lead>
              <LevelBadge level={incident.Level} />
              {/* Two distinct affordances rather than one row that does both: the title
                  opens the full incident, the chevron only peeks at the measurement. */}
              <button
                type='button'
                onClick={() => onOpenDetail(incident)}
                className='cursor-pointer text-xs font-medium text-ink underline decoration-transparent underline-offset-2 transition-colors hover:decoration-current'
              >
                {openedEvent || incident.Kind}
              </button>
              <span className='font-mono text-[0.6875rem] text-ink-dim uppercase'>{incident.Instance}</span>
              {incident.Subject && !incident.Subject.startsWith("application:") && (
                <span className='rounded border border-rim bg-surface-high px-1.5 py-0.5 font-mono text-[0.625rem] text-ink-faint'>
                  {incident.Subject}
                </span>
              )}
            </Lead>
            {/* Only the trigger keeps its detail. It carries the measurement that explains
                why the alert fired at all; the recovery's detail only restates that things
                are fine again, which the row above has already said. */}
            {hasDetail && (
              <>
                <button
                  type='button'
                  onClick={() => setShowDetail((v) => !v)}
                  aria-expanded={showDetail}
                  className='mt-1 flex cursor-pointer items-center gap-1 text-[0.625rem] text-ink-faint transition-colors hover:text-ink-dim'
                >
                  <ChevronRight
                    className={cn("size-3 transition-transform", showDetail && "rotate-90")}
                  />
                  {showDetail ? "Hide detail" : "Show detail"}
                </button>
                {showDetail && (
                  <p className='mt-1 text-[0.6875rem] text-ink-faint'>{incident.OpenedDetail}</p>
                )}
              </>
            )}
          </div>
          <Timestamp at={incident.StartedAt} />
        </div>
      </EventRow>
    </>
  );
}


// The full incident, with its actions. The timeline is built for scanning a day; this is
// where a single incident is read end to end - both events with their details, how long it
// ran, and what can be done about it.
function IncidentDetailModal({
  incident,
  now,
  onClose,
  onResolve,
  resolving,
}: {
  incident: Incident | null;
  now: number;
  onClose: () => void;
  onResolve: (incident: Incident) => void;
  resolving: boolean;
}) {
  if (!incident) return null;

  const resolved = incident.Status === "resolved";
  const tone = levelTone(incident.Level);
  const { event: openedEvent } = splitTitle(incident.OpenedTitle);
  const { event: resolvedEvent } = splitTitle(incident.ResolvedTitle);

  return (
    <Modal open onClose={onClose} title={openedEvent || incident.Kind}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center gap-2'>
          <LevelBadge level={incident.Level} />
          <span
            className={cn(
              "inline-flex items-center gap-1.5 rounded px-1.5 py-0.5 text-[0.625rem] font-semibold uppercase tracking-wide",
              resolved ? "bg-success/10 text-success" : cn("bg-danger/10", tone.text),
            )}
          >
            {resolved ? "Resolved" : "Ongoing"}
          </span>
          <span className='font-mono text-[0.6875rem] text-ink-dim uppercase'>{incident.Instance}</span>
        </div>

        <dl className='divide-y divide-rim rounded-md border border-rim text-xs'>
          <div className='flex items-center justify-between px-3 py-2'>
            <dt className='text-ink-faint'>Condition</dt>
            <dd className='font-mono text-ink'>{incident.Kind}</dd>
          </div>
          {incident.Subject && !incident.Subject.startsWith("application:") && (
            <div className='flex items-center justify-between px-3 py-2'>
              <dt className='text-ink-faint'>Target</dt>
              <dd className='font-mono text-ink'>{incident.Subject}</dd>
            </div>
          )}
          <div className='flex items-center justify-between px-3 py-2'>
            <dt className='text-ink-faint'>Started</dt>
            <dd className='tabular-nums text-ink'>{absoluteTime(incident.StartedAt)}</dd>
          </div>
          <div className='flex items-center justify-between px-3 py-2'>
            <dt className='text-ink-faint'>{resolved ? "Resolved" : "Running for"}</dt>
            <dd className='tabular-nums text-ink'>
              {resolved ? absoluteTime(incident.ResolvedAt) : incidentDuration(incident, now)}
            </dd>
          </div>
          {resolved && (
            <div className='flex items-center justify-between px-3 py-2'>
              <dt className='text-ink-faint'>Duration</dt>
              <dd className='tabular-nums text-ink'>{incidentDuration(incident, now)}</dd>
            </div>
          )}
        </dl>

        {/* Both events in full, newest first, same order as the timeline. */}
        <div className='space-y-2'>
          {resolved && (
            <div className='rounded-md border border-rim px-3 py-2'>
              <p className='flex items-center gap-1.5 text-xs text-ink-dim'>
                <CheckCircle2 className='size-3 shrink-0 text-success' />
                {resolvedEvent || "Recovered"}
              </p>
              {incident.ResolvedDetail && (
                <p className='mt-1 text-[0.6875rem] text-ink-faint'>{incident.ResolvedDetail}</p>
              )}
            </div>
          )}
          <div className='rounded-md border border-rim px-3 py-2'>
            <p className={cn("flex items-center gap-1.5 text-xs", tone.text)}>
              <CircleAlert className='size-3 shrink-0' />
              {openedEvent || incident.Kind}
            </p>
            {incident.OpenedDetail && (
              <p className='mt-1 text-[0.6875rem] text-ink-faint'>{incident.OpenedDetail}</p>
            )}
          </div>
        </div>

        <div className='flex items-center justify-between gap-2'>
          {/* Machine id is 0 for conditions that are not about a host - a failing
              application health check belongs to the project, not to any one machine. */}
          {incident.MachineID > 0 ? (
            <Link
              to={`/hosts/${incident.MachineID}`}
              onClick={onClose}
              className='flex items-center gap-1.5 text-xs text-ink-faint transition-colors hover:text-ink'
            >
              <ExternalLink className='size-3' />
              View host
            </Link>
          ) : (
            <span />
          )}
          <div className='flex items-center gap-2'>
            <Button variant='outline' onClick={onClose}>
              Close
            </Button>
            {!resolved && (
              <Button loading={resolving} disabled={resolving} onClick={() => onResolve(incident)}>
                <Check className='size-3.5' />
                Mark as resolved
              </Button>
            )}
          </div>
        </div>
      </div>
    </Modal>
  );
}

function Placeholder({ children }: { children: React.ReactNode }) {
  return (
    <div className='flex h-[360px] flex-col items-center justify-center gap-2 rounded-lg border border-rim'>
      {children}
    </div>
  );
}

export function IncidentTimeline({
  incidents,
  isLoading,
  isError,
  onResolve,
  resolvingId,
  footer,
}: {
  incidents: Incident[];
  isLoading: boolean;
  isError: boolean;
  onResolve: (incident: Incident) => void;
  resolvingId: number | null;
  // Rendered under the last group: the loading sentinel while more pages exist, or the
  // end-of-feed notice once they run out.
  footer?: React.ReactNode;
}) {
  // One clock for the whole list, so every duration on screen is measured from the same
  // instant. Minute resolution is all fmtDuration renders, so ticking faster would just be
  // re-rendering the same text.
  const now = useNow(30_000);
  const [detail, setDetail] = useState<Incident | null>(null);

  // Kept in sync with the list so a resolution made from inside the modal is reflected
  // there rather than leaving it showing the incident as still running.
  const detailIncident = detail ? incidents.find((i) => i.ID === detail.ID) ?? detail : null;

  if (isLoading) {
    return (
      <div className='space-y-1 rounded-lg border border-rim p-4'>
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className='flex items-center gap-3 py-3 animate-pulse'>
            <div className='size-2.5 shrink-0 rounded-full bg-surface-high' />
            <div className='h-2.5 w-48 rounded bg-surface-high' />
            <div className='ml-auto h-2.5 w-16 rounded bg-surface-high' />
          </div>
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <Placeholder>
        <AlertTriangle className='size-4 text-danger' />
        <p className='text-xs text-danger'>Failed to load incidents.</p>
      </Placeholder>
    );
  }

  if (incidents.length === 0) {
    return (
      <Placeholder>
        <CircleAlert className='size-5 text-ink-faint opacity-30' />
        <p className='text-xs text-ink-faint'>No incidents recorded.</p>
        <p className='max-w-xs text-center text-[0.625rem] text-ink-faint'>
          An incident opens when something fails and closes when it recovers.
        </p>
      </Placeholder>
    );
  }

  const groups = groupByDate(incidents);

  return (
    <div className='space-y-6'>
      {groups.map((group) => (
        <section key={group.key}>
          {/* The heading sits on the spine, which starts at its own centre line and runs
              down to the last entry of the day. */}
          <div className='relative pb-6' style={{ paddingLeft: HEADER_X }}>
            <span
              className='absolute bg-rim-strong'
              style={{ left: SPINE_X - LINE_W / 2, width: LINE_W, top: NODE_CENTER - OVERSHOOT, bottom: 0 }}
            />
            <h2 className='flex items-center gap-2 text-xs font-semibold text-ink-dim' style={{ minHeight: LEAD }}>
              <Clock className='size-3.5 text-ink-faint' />
              {group.label}
            </h2>
          </div>

          {group.incidents.map((incident, i) => (
            <IncidentEntry
              key={incident.ID}
              incident={incident}
              now={now}
              onResolve={onResolve}
              onOpenDetail={setDetail}
              resolving={resolvingId === incident.ID}
              isLastInGroup={i === group.incidents.length - 1}
            />
          ))}
        </section>
      ))}
      {footer}

      <IncidentDetailModal
        incident={detailIncident}
        now={now}
        onClose={() => setDetail(null)}
        onResolve={onResolve}
        resolving={detailIncident ? resolvingId === detailIncident.ID : false}
      />
    </div>
  );
}
