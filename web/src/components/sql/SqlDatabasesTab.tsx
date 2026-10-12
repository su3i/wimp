import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Database } from "lucide-react";
import { prometheusService, type PromInstantResult } from "@/services/prometheus.service";
import { settingService } from "@/services/setting.service";
import { cn } from "@/utils/cn";
import { usePaged } from "@/utils/usePaged";
import { InfoTooltip } from "@/components/ui/InfoTooltip";
import { Pagination } from "@/components/ui/Pagination";
import { fmtBytes, fmtDuration, promSelector, type SqlInstanceGroup } from "@/utils/sql";

// Shortened headers carry their full name behind an info icon.
const HEADERS: [string, string?][] = [
  ["Database"], ["Status"], ["AG", "Availability Group"], ["Data", "Data file size"], ["Log Used", "Transaction log used / log size"],
  ["Tx / sec", "Transactions per second, last 5 min"], ["Last Full Backup", "Last full or differential backup"], ["Last Log Backup"],
];

const TH = "px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";

const STATES: Record<number, string> = {
  0: "Online", 1: "Restoring", 2: "Recovering", 3: "Recovery pending", 4: "Suspect", 5: "Emergency", 6: "Offline",
};

interface Row {
  name: string;
  state?: number;
  ag?: string;
  dataBytes?: number;
  logBytes?: number;
  logUsed?: number;
  tps?: number;
  fullAge?: number;
  logAge?: number;
}

function byDatabase(results: PromInstantResult[], set: (row: Row, v: number, metric: Record<string, string>) => void, rows: Map<string, Row>) {
  for (const r of results) {
    const name = r.metric.database;
    if (!name || name === "_Total" || name === "mssqlsystemresource") continue;
    const row = rows.get(name) ?? { name };
    set(row, parseFloat(r.value[1]), r.metric);
    rows.set(name, row);
  }
}

function Age({ seconds, limit }: { seconds?: number; limit?: number }) {
  if (seconds == null) return <span className='text-xs text-ink-faint'>N/A</span>;
  const overdue = limit != null && seconds > limit;
  return (
    <span className={cn("text-xs tabular-nums", overdue ? "text-danger font-medium" : "text-ink-dim")}>
      {fmtDuration(seconds)} ago
    </span>
  );
}

// "109 MB / 136 MB (80%)": used is derived from the log size and the percentage SQL Server reports.
function LogUsed({ bytes, pct }: { bytes?: number; pct?: number }) {
  const text =
    pct == null ? (bytes != null ? fmtBytes(bytes) : "N/A")
    : bytes == null ? `${pct.toFixed(0)}%`
    : `${fmtBytes((bytes * pct) / 100)} / ${fmtBytes(bytes)} (${pct.toFixed(0)}%)`;
  return <span className='text-xs text-ink-dim tabular-nums whitespace-nowrap'>{text}</span>;
}

export function SqlDatabasesTab({ group }: { group: SqlInstanceGroup }) {
  const a = promSelector(group, "mssql_instance");
  const b = promSelector(group, "sql_instance");

  const { data: thresholds } = useQuery({
    queryKey: ["settings"],
    queryFn: settingService.get,
    select: (d) => d.settings.sql_thresholds,
    staleTime: 60_000,
  });

  const { data: rows, isLoading, isError } = useQuery({
    queryKey: ["sql-databases", group.key],
    enabled: prometheusService.isConfigured(),
    refetchInterval: 30_000,
    queryFn: async () => {
      const q = (query: string) => prometheusService.instant(query).catch(() => [] as PromInstantResult[]);
      const [data, log, logUsed, tps, state, backups] = await Promise.all([
        q(`max by (database) (windows_mssql_databases_data_files_size_bytes{${a}})`),
        q(`max by (database) (windows_mssql_databases_log_files_size_bytes{${a}})`),
        q(`max by (database) (windows_mssql_databases_log_used_percent{${a}})`),
        q(`sum by (database) (rate(windows_mssql_databases_transactions{${a}}[5m]))`),
        q(`max by (database, availability_group) (wimp_mssql_database_state{${b}})`),
        q(`max by (database, kind) (wimp_mssql_database_backup_age_seconds{${b}})`),
      ]);
      const m = new Map<string, Row>();
      byDatabase(data, (r, v) => (r.dataBytes = v), m);
      byDatabase(log, (r, v) => (r.logBytes = v), m);
      byDatabase(logUsed, (r, v) => (r.logUsed = v), m);
      byDatabase(tps, (r, v) => (r.tps = v), m);
      byDatabase(state, (r, v, metric) => {
        r.state = v;
        r.ag = metric.availability_group || undefined;
      }, m);
      byDatabase(backups, (r, v, metric) => {
        if (metric.kind === "full") r.fullAge = v;
        else if (metric.kind === "log") r.logAge = v;
      }, m);
      return [...m.values()];
    },
  });

  const fullLimit = thresholds ? thresholds.backup_hours * 3600 : undefined;
  const logLimit = thresholds ? thresholds.log_backup_minutes * 60 : undefined;

  // Anything needing attention first: a database not online, then an overdue backup.
  const sorted = [...(rows ?? [])].sort((x, y) => {
    const rank = (r: Row) =>
      (r.state != null && r.state !== 0 ? 2 : 0) +
      ((fullLimit != null && (r.fullAge ?? 0) > fullLimit) || (logLimit != null && (r.logAge ?? 0) > logLimit) ? 1 : 0);
    return rank(y) - rank(x) || x.name.localeCompare(y.name);
  });

  const paged = usePaged(sorted);

  if (isLoading) return <div className='h-40 rounded-lg border border-rim bg-surface animate-pulse' />;
  if (isError) {
    return (
      <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
        <AlertCircle className='size-4 shrink-0' />
        Failed to load databases.
      </div>
    );
  }
  if (!sorted.length) {
    return (
      <div className='flex flex-col items-center justify-center py-16 rounded-lg border border-rim bg-surface text-center'>
        <Database className='size-5 text-ink-faint mb-3' />
        <p className='text-xs text-ink-faint'>No database metrics reported for this instance yet.</p>
      </div>
    );
  }

  // A real table rather than a grid per row: every row shares one set of column widths, so
  // cells can size to their content, never wrap, and the table scrolls sideways instead.
  return (
    <div className='rounded-lg border border-rim overflow-x-auto'>
      <table className='w-full whitespace-nowrap'>
        <thead className='bg-surface-alt'>
          <tr className='border-b border-rim'>
            {HEADERS.map(([h, full]) => (
              <th key={h} className={TH}>
                {full ? (
                  <span className='inline-flex items-center gap-1.5'>
                    {h}
                    <InfoTooltip text={full} />
                  </span>
                ) : (
                  h
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {paged.items.map((r) => {
            const online = r.state == null || r.state === 0;
            return (
              <tr key={r.name} className='border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'>
                <td className='px-4 py-3'>
                  <div className='flex items-center gap-3'>
                    <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                      <Database className='size-3 text-ink-faint' />
                    </div>
                    <span className='font-mono text-xs text-ink'>{r.name}</span>
                  </div>
                </td>
                <td className='px-4 py-3'>
                  {r.state == null ? (
                    <span className='text-xs text-ink-faint'>N/A</span>
                  ) : (
                    <span className={cn("inline-flex items-center gap-2 text-xs font-medium", online ? "text-success" : "text-danger")}>
                      <span className={cn("size-1.5 rounded-full", online ? "bg-success" : "bg-danger")} />
                      {STATES[r.state] ?? `State ${r.state}`}
                    </span>
                  )}
                </td>
                <td className='px-4 py-3 font-mono text-xs text-ink-dim'>{r.ag ?? <span className='font-sans text-ink-faint'>None</span>}</td>
                <td className='px-4 py-3 text-xs text-ink-dim tabular-nums'>{r.dataBytes != null ? fmtBytes(r.dataBytes) : "N/A"}</td>
                <td className='px-4 py-3'><LogUsed bytes={r.logBytes} pct={r.logUsed} /></td>
                <td className='px-4 py-3 text-xs text-ink-dim tabular-nums'>{r.tps != null ? r.tps.toFixed(1) : "N/A"}</td>
                <td className='px-4 py-3'><Age seconds={r.fullAge} limit={fullLimit} /></td>
                <td className='px-4 py-3'>
                  {r.fullAge != null && r.logAge == null ? (
                    <span className='text-xs text-ink-faint'>Simple recovery</span>
                  ) : (
                    <Age seconds={r.logAge} limit={logLimit} />
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {paged.totalPages > 1 && (
        <div className='border-t border-rim px-4 py-2.5'>
          <Pagination page={paged.page} totalPages={paged.totalPages} total={paged.total} pageSize={paged.pageSize} onPageChange={paged.setPage} />
        </div>
      )}
    </div>
  );
}
