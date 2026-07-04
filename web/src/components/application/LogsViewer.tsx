import { useState, useMemo, useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw, Terminal, ChevronDown, Maximize2, Minimize2, Download } from "lucide-react";
import { toast } from "sonner";
import { logsService } from "@/services/logs.service";
import { cn } from "@/utils/cn";

// ── Types ─────────────────────────────────────────────────────────────────────

interface Machine {
  id: number;
  hostname: string;
}

interface Pool {
  id: number;
  name: string;
  machineId: number;
}

interface LogEntry {
  ts: number;
  content: string;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const TIME_RANGES = [
  { label: "Last 1h", value: "1h", ms: 3_600_000 },
  { label: "Last 6h", value: "6h", ms: 21_600_000 },
  { label: "Last 24h", value: "24h", ms: 86_400_000 },
  { label: "Last 7d", value: "7d", ms: 604_800_000 },
] as const;

type TimeRange = (typeof TIME_RANGES)[number]["value"];

const LIMITS = [50, 100, 500, 1000] as const;

function getRangeParams(range: TimeRange) {
  const now = new Date();
  const ms = TIME_RANGES.find((r) => r.value === range)!.ms;
  return { end: now.toISOString(), start: new Date(now.getTime() - ms).toISOString() };
}

function fmtLogTime(nanoTs: string): string {
  const ms = Number(BigInt(nanoTs) / 1_000_000n);
  const d = new Date(ms);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  const ms3 = String(d.getMilliseconds()).padStart(3, "0");
  return `${hh}:${mm}:${ss}.${ms3}`;
}

function basename(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

function extractLogLine(raw: string): string {
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    if (typeof parsed.log === "string") return parsed.log;
  } catch {
    // not JSON - use raw
  }
  return raw;
}

// ── Select ────────────────────────────────────────────────────────────────────

function FilterSelect<T extends string | number>({
  value,
  onChange,
  options,
  disabled,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { label: string; value: T }[];
  disabled?: boolean;
}) {
  return (
    <div className='relative'>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value as T)}
        disabled={disabled}
        className='appearance-none h-7 pl-2.5 pr-6 rounded-md border border-rim bg-surface text-xs text-ink cursor-pointer focus:outline-none focus:border-accent disabled:opacity-50 disabled:cursor-not-allowed'
      >
        {options.map((o) => (
          <option key={String(o.value)} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      <ChevronDown className='pointer-events-none absolute right-1.5 top-1/2 -translate-y-1/2 size-3 text-ink-faint' />
    </div>
  );
}

// ── Component ─────────────────────────────────────────────────────────────────

export function LogsViewer({
  projectKey,
  appId,
  machines,
  pools,
}: {
  projectKey: string;
  appId: number;
  machines: Machine[];
  pools: Pool[];
}) {
  const [machineId, setMachineId] = useState<string>(() =>
    machines.length > 0 ? String(machines[0].id) : "",
  );
  const [poolId, setPoolId] = useState<string>(() => {
    if (machines.length === 0) return "";
    const firstPool = pools.find((p) => p.machineId === machines[0].id);
    return firstPool ? String(firstPool.id) : "";
  });
  const [filename, setFilename] = useState<string>("");
  const [timeRange, setTimeRange] = useState<TimeRange>("1h");
  const [limit, setLimit] = useState<number>(100);
  const [atBottom, setAtBottom] = useState(true);
  const [expanded, setExpanded] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // When machine changes, reset pool selection
  function handleMachineChange(v: string) {
    setMachineId(v);
    setPoolId("");
  }

  // Pool options filtered by selected machine
  const poolOptions = useMemo(() => {
    const filtered = machineId ? pools.filter((p) => p.machineId === Number(machineId)) : pools;
    return [
      { label: "App Pool…", value: "" },
      ...filtered.map((p) => ({ label: p.name, value: String(p.id) })),
    ];
  }, [pools, machineId]);

  const machineOptions = useMemo(
    () => [
      { label: "Host…", value: "" },
      ...machines.map((m) => ({ label: m.hostname, value: String(m.id) })),
    ],
    [machines],
  );

  // Fetch available log files
  const { data: filesData, isLoading: filesLoading } = useQuery({
    queryKey: ["app-files", projectKey, appId],
    queryFn: async () => {
      const resp = await logsService.listFiles(projectKey, appId);
      return resp.data.files ?? [];
    },
    staleTime: 30_000,
  });

  const fileOptions = useMemo(
    () => (filesData ?? []).map((f) => ({ label: basename(f), value: f })),
    [filesData],
  );

  // Auto-select first file on initial load
  useEffect(() => {
    if (!filename && fileOptions.length > 0) {
      setFilename(fileOptions[0].value);
    }
  }, [filename, fileOptions]);

  // Clear filename if it's been removed from the available list
  useEffect(() => {
    if (filename && filesData && !filesData.includes(filename)) {
      setFilename("");
    }
  }, [filesData, filename]);

  const params = useMemo(() => {
    const { start, end } = getRangeParams(timeRange);
    return {
      start,
      end,
      limit,
      direction: "backward" as const,
      ...(machineId ? { machine_id: Number(machineId) } : {}),
      ...(poolId ? { pool_id: Number(poolId) } : {}),
      ...(filename ? { filename } : {}),
    };
  }, [machineId, poolId, filename, timeRange, limit]);

  const { data, isLoading, isError, isFetching, refetch } = useQuery({
    queryKey: ["logs", projectKey, appId, params],
    queryFn: async () => {
      const resp = await logsService.query(projectKey, appId, params);
      return resp.data;
    },
    staleTime: 0,
  });

  const entries = useMemo<LogEntry[]>(() => {
    const result: LogEntry[] = [];
    for (const stream of data?.data?.result ?? []) {
      for (const [nanoTs, content] of stream.values) {
        result.push({
          ts: Number(BigInt(nanoTs) / 1_000_000n),
          content: extractLogLine(content),
        });
      }
    }
    return result.sort((a, b) => a.ts - b.ts);
  }, [data]);

  useEffect(() => {
    if (atBottom && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [entries, atBottom]);

  function handleScroll() {
    const el = scrollRef.current;
    if (!el) return;
    setAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 40);
  }

  async function handleDownload() {
    if (!machineId || !filename) {
      toast.error("Select a host and log file before downloading.");
      return;
    }
    setDownloading(true);
    try {
      const resp = await logsService.downloadLogs(projectKey, Number(machineId), filename);
      const blob = new Blob([resp.data as BlobPart], { type: "application/zip" });
      const url = URL.createObjectURL(blob);
      const hostname = machines.find((m) => m.id === Number(machineId))?.hostname ?? String(machineId);
      const a = document.createElement("a");
      a.href = url;
      a.download = `logs-${hostname}.zip`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to download logs.";
      toast.error(msg);
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div
      className={cn(
        "flex flex-col rounded-lg border border-rim overflow-hidden",
        expanded ? "fixed left-14 inset-y-0 right-0 z-[9999]" : "h-[50vh]",
      )}
    >
      {/* Filter bar */}
      <div className='flex items-center gap-2 px-4 py-2.5 border-b border-rim bg-surface-alt shrink-0 flex-wrap'>
        <Terminal className='size-3.5 text-ink-faint shrink-0' />
        <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint mr-1'>Logs</span>

        <FilterSelect
          value={machineId}
          onChange={(v) => handleMachineChange(String(v))}
          options={machineOptions}
        />

        <FilterSelect
          value={poolId}
          onChange={(v) => setPoolId(String(v))}
          options={poolOptions}
          disabled={!pools.length}
        />

        <FilterSelect
          value={filename}
          onChange={(v) => setFilename(String(v))}
          disabled={filesLoading || !fileOptions.length}
          options={
            filesLoading
              ? [{ label: "Loading files…", value: "" }]
              : !fileOptions.length
                ? [{ label: "No files configured", value: "" }]
                : fileOptions
          }
        />

        <FilterSelect
          value={timeRange}
          onChange={(v) => setTimeRange(v as TimeRange)}
          options={TIME_RANGES.map((r) => ({ label: r.label, value: r.value }))}
        />

        <FilterSelect
          value={limit}
          onChange={(v) => setLimit(Number(v))}
          options={LIMITS.map((l) => ({ label: String(l), value: l }))}
        />

        <div className='ml-auto flex items-center gap-1.5'>
          <button
            type='button'
            onClick={() => void handleDownload()}
            disabled={downloading || !machineId || !filename}
            className='flex items-center gap-1.5 h-7 px-2.5 rounded-md border border-rim bg-surface text-xs text-ink-faint hover:text-ink transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed'
            title='Download logs as zip'
          >
            <Download className={cn("size-3", downloading && "animate-pulse")} />
            {downloading ? "Downloading…" : "Download"}
          </button>
          <button
            type='button'
            onClick={() => void refetch()}
            className='flex items-center gap-1.5 h-7 px-2.5 rounded-md border border-rim bg-surface text-xs text-ink-faint hover:text-ink transition-colors cursor-pointer'
          >
            <RefreshCw className={cn("size-3", isFetching && "animate-spin")} />
            Refresh
          </button>
          <button
            type='button'
            onClick={() => setExpanded((v) => !v)}
            className='flex items-center justify-center h-7 w-7 rounded-md border border-rim bg-surface text-ink-faint hover:text-ink transition-colors cursor-pointer'
            title={expanded ? "Collapse" : "Expand"}
          >
            {expanded ? <Minimize2 className='size-3.5' /> : <Maximize2 className='size-3.5' />}
          </button>
        </div>
      </div>

      {/* Log area */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className='flex-1 overflow-y-auto bg-canvas relative'
        style={{ fontFamily: "ui-monospace, 'SF Mono', Menlo, Monaco, monospace", fontSize: "11px" }}
      >
        {isLoading ? (
          <div className='flex items-center justify-center h-full text-ink-faint text-xs'>Loading logs…</div>
        ) : isError ? (
          <div className='flex items-center justify-center h-full text-danger text-xs'>
            Failed to load logs.
          </div>
        ) : !entries.length ? (
          <div className='flex flex-col items-center justify-center h-full gap-2 text-ink-faint text-xs'>
            <Terminal className='size-5 opacity-30' />
            <span>No log entries found for the selected filters.</span>
          </div>
        ) : (
          <div className='p-2'>
            {entries.map((entry, i) => (
              <div
                key={i}
                className='flex items-baseline gap-1.5 px-2 py-[2px] rounded hover:bg-surface-high/50 leading-relaxed'
              >
                <span className='shrink-0 text-ink-faint w-24'>
                  {fmtLogTime(String(BigInt(entry.ts) * 1_000_000n))}
                </span>
                <span className='text-ink-dim break-all flex-1'>{entry.content}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
