import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, CalendarClock, Check, Copy, Info, Pause, Play, RefreshCw, Workflow } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Pagination } from "@/components/ui/Pagination";
import { sqlInstanceService } from "@/services/sqlInstance.service";
import { cn } from "@/utils/cn";
import { usePaged } from "@/utils/usePaged";
import { apiError, fmtDuration, type SqlInstanceGroup } from "@/utils/sql";
import type { SqlLiveKind } from "@/types";

const TH = "px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";
const TD = "px-4 py-3 text-xs text-ink-dim";

type LiveRow = Record<string, unknown>;

function useLive(projectKey: string, group: SqlInstanceGroup, kind: SqlLiveKind, refetchMs: number | false) {
  return useQuery({
    queryKey: ["sql-live", projectKey, group.key, kind],
    retry: false,
    refetchInterval: refetchMs,
    queryFn: async () =>
      (await sqlInstanceService.live(projectKey, group.current.MachineID, group.current.Name, kind)).data.rows ?? [],
  });
}

const str = (v: unknown) => (v === null || v === undefined ? "" : String(v));
const numv = (v: unknown) => (typeof v === "number" ? v : Number(v));

function Panel({
  title, sub, icon: Icon, actions, children,
}: { title: string; sub?: string; icon: typeof Workflow; actions?: ReactNode; children: ReactNode }) {
  return (
    <div className='rounded-lg border border-rim overflow-hidden'>
      <div className='flex items-center justify-between gap-4 border-b border-rim bg-surface-alt px-4 py-2.5'>
        <div className='min-w-0'>
          <p className='flex items-center gap-2 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
            <Icon className='size-3' />
            {title}
          </p>
          {sub && <p className='mt-1 text-[0.6875rem] text-ink-faint'>{sub}</p>}
        </div>
        <div className='flex shrink-0 items-center gap-3'>{actions}</div>
      </div>
      {children}
    </div>
  );
}

function Refresh({ fetching, onClick }: { fetching: boolean; onClick: () => void }) {
  return (
    <button type='button' onClick={onClick} className='flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink cursor-pointer'>
      <RefreshCw className={cn("size-3", fetching && "animate-spin")} />
      Refresh
    </button>
  );
}

function Message({ children, tone }: { children: ReactNode; tone?: "error" }) {
  return (
    <p className={cn("flex items-center gap-2 bg-surface px-4 py-6 text-xs justify-center", tone === "error" ? "text-danger" : "text-ink-faint")}>
      {tone === "error" && <AlertCircle className='size-3.5 shrink-0' />}
      {children}
    </p>
  );
}

// ── Table ─────────────────────────────────────────────────────────────────────

interface Column {
  header: string;
  cell: (row: LiveRow) => ReactNode;
}

// One-line cells, columns sized to their content, sideways scroll when too wide, and ten rows
// a page, the same as the Databases table.
function LiveTable({ columns, rows }: { columns: Column[]; rows: LiveRow[] }) {
  const paged = usePaged(rows);
  return (
    <>
      <div className='overflow-x-auto bg-surface'>
        <table className='w-full whitespace-nowrap'>
          <thead>
            <tr className='border-b border-rim'>
              {columns.map((c) => (
                <th key={c.header} className={TH}>{c.header}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {paged.items.map((row, i) => (
              <tr key={i} className='border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'>
                {columns.map((c) => (
                  <td key={c.header} className={TD}>{c.cell(row)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {paged.totalPages > 1 && (
        <div className='border-t border-rim bg-surface px-4 py-2.5'>
          <Pagination page={paged.page} totalPages={paged.totalPages} total={paged.total} pageSize={paged.pageSize} onPageChange={paged.setPage} />
        </div>
      )}
    </>
  );
}

// Long text shortened to one line, with an info icon that opens all of it.
function LongText({ text, mono, onOpen }: { text: unknown; mono?: boolean; onOpen: () => void }) {
  const t = str(text).replace(/\s+/g, " ").trim();
  if (!t) return <span className='text-ink-faint'>N/A</span>;
  return (
    <span className='flex items-center gap-1.5'>
      <span className={cn("block max-w-[26rem] truncate", mono && "font-mono text-[0.6875rem]")}>{t}</span>
      <button type='button' onClick={onOpen} title='Show in full' className='shrink-0 text-ink-faint hover:text-ink cursor-pointer'>
        <Info className='size-3' />
      </button>
    </span>
  );
}

interface FullText {
  title: string;
  text: string;
  sql?: boolean;
}

// The whole of a statement or message, as the server returned it, with a copy button.
function FullTextModal({ view, onClose }: { view: FullText | null; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    await navigator.clipboard.writeText(view?.text ?? "");
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }
  return (
    <Modal open={!!view} onClose={onClose} title={view?.title ?? ""} className='max-w-[760px]'>
      <div className='space-y-3'>
        <pre
          className={cn(
            "max-h-[60vh] overflow-auto rounded-md border border-rim bg-canvas p-3 text-[0.6875rem] leading-relaxed text-ink-dim whitespace-pre-wrap break-words",
            view?.sql && "font-mono",
          )}
        >
          {view?.text.trim()}
        </pre>
        <div className='flex justify-end gap-2'>
          <Button variant='outline' onClick={() => void copy()}>
            {copied ? <Check className='size-3.5' /> : <Copy className='size-3.5' />}
            {copied ? "Copied" : "Copy"}
          </Button>
          <Button onClick={onClose}>Close</Button>
        </div>
      </div>
    </Modal>
  );
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

const OUTCOME: Record<number, { label: string; tone: string; dot: string }> = {
  0: { label: "Failed", tone: "text-danger", dot: "bg-danger" },
  1: { label: "Succeeded", tone: "text-success", dot: "bg-success" },
  2: { label: "Retrying", tone: "text-warning", dot: "bg-warning" },
  3: { label: "Canceled", tone: "text-ink-dim", dot: "bg-ink-faint" },
  4: { label: "Running", tone: "text-primary", dot: "bg-primary" },
};

// sysjobhistory stores run duration as an HHMMSS integer.
function hhmmss(v: unknown) {
  const n = numv(v);
  if (!Number.isFinite(n)) return "N/A";
  return fmtDuration(Math.floor(n / 10000) * 3600 + (Math.floor(n / 100) % 100) * 60 + (n % 100));
}

// Offsets come from the server's own clock (see the agent's jobs query), so they are right
// whatever time zone the server and the viewer are in.
function lastRun(seconds: unknown) {
  return seconds == null ? "Never" : `${fmtDuration(numv(seconds))} ago`;
}

function nextRun(seconds: unknown) {
  if (seconds == null) return "Not scheduled";
  const s = numv(seconds);
  return s <= 0 ? "Due now" : `in ${fmtDuration(s)}`;
}

export function SqlJobsTab({ projectKey, group }: { projectKey: string; group: SqlInstanceGroup }) {
  const { data, isLoading, isError, error, isFetching, refetch } = useLive(projectKey, group, "jobs", 60_000);
  const [view, setView] = useState<FullText | null>(null);

  // Failing first, then disabled last, otherwise by name.
  const jobs = [...(data ?? [])].sort((x, y) => {
    const rank = (r: LiveRow) => (numv(r.last_run_status) === 0 ? 0 : r.enabled ? 1 : 2);
    return rank(x) - rank(y) || str(x.name).localeCompare(str(y.name));
  });

  const columns: Column[] = [
    {
      header: "Job",
      cell: (j) => (
        <span className={j.enabled ? "text-ink" : "text-ink-faint"}>
          {str(j.name)}
          {!j.enabled && <span className='ml-2 text-[0.625rem]'>Disabled</span>}
        </span>
      ),
    },
    { header: "Category", cell: (j) => str(j.category) || "N/A" },
    {
      header: "Last Outcome",
      cell: (j) => {
        const status = j.last_run_status == null ? null : OUTCOME[numv(j.last_run_status)];
        return status ? (
          <span className={cn("inline-flex items-center gap-2 font-medium", status.tone)}>
            <span className={cn("size-1.5 rounded-full", status.dot)} />
            {status.label}
          </span>
        ) : (
          <span className='text-ink-faint'>Never run</span>
        );
      },
    },
    { header: "Last Run", cell: (j) => lastRun(j.last_run_ago_seconds) },
    { header: "Duration", cell: (j) => <span className='tabular-nums'>{j.last_run_at ? hhmmss(j.last_run_duration_hhmmss) : "N/A"}</span> },
    { header: "Next Run", cell: (j) => nextRun(j.next_run_in_seconds) },
    {
      header: "Message",
      cell: (j) => <LongText text={j.last_message} onOpen={() => setView({ title: `${str(j.name)}: last run`, text: str(j.last_message) })} />,
    },
  ];

  return (
    <>
      <Panel title='SQL Agent Jobs' icon={CalendarClock} actions={<Refresh fetching={isFetching} onClick={() => void refetch()} />}>
        {isLoading ? (
          <div className='h-32 bg-surface animate-pulse' />
        ) : isError ? (
          <Message tone='error'>{apiError(error, "Could not read jobs.")}</Message>
        ) : !jobs.length ? (
          <Message>No SQL Agent jobs on this instance.</Message>
        ) : (
          <LiveTable columns={columns} rows={jobs} />
        )}
      </Panel>
      <FullTextModal view={view} onClose={() => setView(null)} />
    </>
  );
}

// ── Live queries ──────────────────────────────────────────────────────────────

export function SqlActivityTab({ projectKey, group }: { projectKey: string; group: SqlInstanceGroup }) {
  const [live, setLive] = useState(true);
  const [view, setView] = useState<FullText | null>(null);
  // Hold still while a statement is open, so the rows do not shift under the reader.
  const interval = live && !view ? 10_000 : false;
  const blocking = useLive(projectKey, group, "blocking", interval);
  const requests = useLive(projectKey, group, "requests", interval);

  const statement = (r: LiveRow) => (
    <LongText text={r.sql_text} mono onOpen={() => setView({ title: `Session ${str(r.session_id)}`, text: str(r.sql_text), sql: true })} />
  );

  const blockingColumns: Column[] = [
    { header: "Session", cell: (r) => <span className='font-mono text-ink'>{str(r.session_id)}</span> },
    { header: "Blocked By", cell: (r) => <span className='font-mono text-danger'>{str(r.blocking_session_id)}</span> },
    { header: "Waiting", cell: (r) => <span className='tabular-nums'>{fmtDuration(numv(r.wait_ms) / 1000)}</span> },
    { header: "Wait Type", cell: (r) => str(r.wait_type) || "N/A" },
    { header: "Database", cell: (r) => str(r.database_name) || "N/A" },
    { header: "Login", cell: (r) => str(r.login_name) || "N/A" },
    { header: "Blocker Login", cell: (r) => str(r.blocker_login) || "N/A" },
    { header: "Blocker Host", cell: (r) => str(r.blocker_host) || "N/A" },
    { header: "Statement", cell: statement },
  ];

  const requestColumns: Column[] = [
    { header: "Session", cell: (r) => <span className='font-mono text-ink'>{str(r.session_id)}</span> },
    { header: "Status", cell: (r) => <span className='capitalize'>{str(r.status) || "N/A"}</span> },
    { header: "Command", cell: (r) => str(r.command) || "N/A" },
    { header: "Elapsed", cell: (r) => <span className='tabular-nums'>{fmtDuration(numv(r.elapsed_ms) / 1000)}</span> },
    { header: "CPU", cell: (r) => <span className='tabular-nums'>{fmtDuration(numv(r.cpu_ms) / 1000)}</span> },
    { header: "Wait Type", cell: (r) => str(r.wait_type) || "None" },
    { header: "Database", cell: (r) => str(r.database_name) || "N/A" },
    { header: "Login", cell: (r) => str(r.login_name) || "N/A" },
    { header: "Host", cell: (r) => str(r.host_name) || "N/A" },
    { header: "Statement", cell: statement },
  ];

  const toggle = (
    <button type='button' onClick={() => setLive(!live)} className='flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink cursor-pointer'>
      {live ? <Pause className='size-3' /> : <Play className='size-3' />}
      {live ? "Pause" : "Resume"}
    </button>
  );

  return (
    <div className='space-y-4'>
      <Panel
        title={`Blocked Queries${blocking.data?.length ? ` · ${blocking.data.length}` : ""}`}
        sub="Queries stuck waiting for another query to release a lock."
        icon={AlertCircle}
        actions={<>{toggle}<Refresh fetching={blocking.isFetching} onClick={() => void blocking.refetch()} /></>}
      >
        {blocking.isLoading ? (
          <div className='h-20 bg-surface animate-pulse' />
        ) : blocking.isError ? (
          <Message tone='error'>{apiError(blocking.error, "Could not read blocking.")}</Message>
        ) : !blocking.data?.length ? (
          <Message>No query is waiting on another right now.</Message>
        ) : (
          <LiveTable columns={blockingColumns} rows={blocking.data} />
        )}
      </Panel>

      <Panel
        title='Running Queries'
        sub="Every query executing on the instance right now, longest-running first."
        icon={Workflow}
        actions={<Refresh fetching={requests.isFetching} onClick={() => void requests.refetch()} />}
      >
        {requests.isLoading ? (
          <div className='h-20 bg-surface animate-pulse' />
        ) : requests.isError ? (
          <Message tone='error'>{apiError(requests.error, "Could not read requests.")}</Message>
        ) : !requests.data?.length ? (
          <Message>No queries are running right now.</Message>
        ) : (
          <LiveTable columns={requestColumns} rows={requests.data} />
        )}
      </Panel>

      <FullTextModal view={view} onClose={() => setView(null)} />
    </div>
  );
}
