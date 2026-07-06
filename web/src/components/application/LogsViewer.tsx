import { useState, useMemo, useEffect, useRef, useLayoutEffect } from "react";
import { createPortal } from "react-dom";
import { useQuery } from "@tanstack/react-query";
import {
  RefreshCw,
  Terminal,
  ChevronDown,
  Maximize2,
  Minimize2,
  Download,
  FileArchive,
  Trash2,
  X,
  Search,
} from "lucide-react";
import { toast } from "sonner";
import { logsService } from "@/services/logs.service";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
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
  ts: number;    // milliseconds
  nanoTs: string; // original nanosecond string, used for pagination
  content: string;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtLogTime(nanoTs: string): string {
  const ms = Number(BigInt(nanoTs) / 1_000_000n);
  const d = new Date(ms);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  const ms3 = String(d.getMilliseconds()).padStart(3, "0");
  return `${hh}:${mm}:${ss}.${ms3}`;
}

function highlight(text: string, query: string): React.ReactNode {
  const esc = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const parts = text.split(new RegExp(`(${esc})`, "gi"));
  return (
    <>
      {parts.map((part, i) =>
        i % 2 === 1 ? (
          <mark key={i} className="bg-[#f5c842]/35 text-inherit rounded-[2px]">{part}</mark>
        ) : (
          part
        )
      )}
    </>
  );
}

function basename(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

function extractLogLine(raw: string): string {
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    if (typeof parsed.log === "string") return parsed.log;
  } catch {
    // not JSON — use raw
  }
  return raw;
}

function extractEntries(data: { result?: { values: [string, string][] }[] } | undefined): LogEntry[] {
  const out: LogEntry[] = [];
  for (const stream of data?.result ?? []) {
    for (const [nanoTs, content] of stream.values) {
      out.push({
        ts: Number(BigInt(nanoTs) / 1_000_000n),
        nanoTs,
        content: extractLogLine(content),
      });
    }
  }
  return out;
}

const FAR_BACK = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString();
const FETCH_LIMIT = 1000;

// ── Tooltip ───────────────────────────────────────────────────────────────────

function Tooltip({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className='relative group/tip'>
      {children}
      <div className='pointer-events-none absolute top-full left-1/2 -translate-x-1/2 mt-1.5 px-2 py-1 rounded border border-rim bg-surface-highest text-[0.625rem] text-ink whitespace-nowrap opacity-0 group-hover/tip:opacity-100 transition-opacity z-50 shadow-md'>
        {label}
      </div>
    </div>
  );
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
  const [machineId, setMachineId] = useState("");
  const [poolId, setPoolId] = useState("");
  const [filename, setFilename] = useState("");
  const [searchText, setSearchText] = useState("");
  const [expanded, setExpanded] = useState(false);

  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [isInitialLoading, setIsInitialLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [initialLoaded, setInitialLoaded] = useState(false);
  const [fetchError, setFetchError] = useState(false);
  const [fetchKey, setFetchKey] = useState(0);

  const [staging, setStaging] = useState(false);
  const [fetchingDownload, setFetchingDownload] = useState(false);
  const [pendingDownload, setPendingDownload] = useState<
    { token: string; fileName: string; fileSize: number } | null
  >(null);
  const [clearConfirmOpen, setClearConfirmOpen] = useState(false);
  const [clearing, setClearing] = useState(false);

  const scrollRef = useRef<HTMLDivElement>(null);
  const prevScrollHeightRef = useRef(0);
  const loadingMoreRef = useRef(false);

  // ── Selectors ───────────────────────────────────────────────────────────────

  function handleMachineChange(v: string) {
    setMachineId(v);
    setPoolId("");
    setFilename("");
  }

  const machineOptions = useMemo(
    () => [
      { label: "Select host…", value: "" },
      ...machines.map((m) => ({ label: m.hostname, value: String(m.id) })),
    ],
    [machines],
  );

  const poolOptions = useMemo(() => {
    const filtered = machineId ? pools.filter((p) => p.machineId === Number(machineId)) : pools;
    return [
      { label: "Select app pool…", value: "" },
      ...filtered.map((p) => ({ label: p.name, value: String(p.id) })),
    ];
  }, [pools, machineId]);

  // ── Files (machine-scoped) ─────────────────────────────────────────────────

  const { data: filesData, isLoading: filesLoading } = useQuery({
    queryKey: ["app-files", projectKey, appId, machineId],
    queryFn: async () => {
      const resp = await logsService.listFiles(projectKey, appId, Number(machineId));
      return resp.data.files ?? [];
    },
    enabled: !!machineId,
    staleTime: 30_000,
  });

  const fileOptions = useMemo(
    () => (filesData ?? []).map((f) => ({ label: basename(f), value: f })),
    [filesData],
  );

  // Clear filename if it disappeared from the available list
  useEffect(() => {
    if (filename && filesData && !filesData.includes(filename)) {
      setFilename("");
    }
  }, [filesData, filename]);

  // ── Log fetching ───────────────────────────────────────────────────────────

  const readyToFetch = !!machineId && !!poolId;

  useEffect(() => {
    if (!readyToFetch) {
      setEntries([]);
      setHasMore(false);
      setInitialLoaded(false);
      setIsInitialLoading(false);
      setFetchError(false);
      return;
    }

    let cancelled = false;
    setEntries([]);
    setHasMore(false);
    setInitialLoaded(false);
    setFetchError(false);
    setIsInitialLoading(true);

    void (async () => {
      try {
        const resp = await logsService.query(projectKey, appId, {
          start: FAR_BACK,
          end: new Date().toISOString(),
          limit: FETCH_LIMIT,
          direction: "backward",
          machine_id: Number(machineId),
          pool_id: Number(poolId),
          ...(filename ? { filename } : {}),
        });
        if (cancelled) return;
        const result = extractEntries(resp.data.data);
        result.sort((a, b) => a.ts - b.ts);
        setEntries(result);
        setHasMore(result.length === FETCH_LIMIT);
        setInitialLoaded(true);
      } catch {
        if (!cancelled) {
          setFetchError(true);
          setInitialLoaded(true);
        }
      } finally {
        if (!cancelled) setIsInitialLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [readyToFetch, machineId, poolId, filename, projectKey, appId, fetchKey]);

  // Scroll to bottom after initial load and whenever the viewer is expanded/collapsed
  useEffect(() => {
    if (initialLoaded && !fetchError && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [initialLoaded, fetchError, expanded]);

  // Restore scroll position after prepending older entries
  useLayoutEffect(() => {
    if (prevScrollHeightRef.current > 0 && scrollRef.current) {
      scrollRef.current.scrollTop += scrollRef.current.scrollHeight - prevScrollHeightRef.current;
      prevScrollHeightRef.current = 0;
    }
  }, [entries]);

  async function loadMore() {
    if (loadingMoreRef.current || !hasMore || entries.length === 0) return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    prevScrollHeightRef.current = scrollRef.current?.scrollHeight ?? 0;

    const oldestNano = entries[0].nanoTs;
    const endNs = BigInt(oldestNano) - 1n;
    const endMs = Number(endNs / 1_000_000n);

    try {
      const resp = await logsService.query(projectKey, appId, {
        start: FAR_BACK,
        end: new Date(endMs).toISOString(),
        limit: FETCH_LIMIT,
        direction: "backward",
        machine_id: Number(machineId),
        pool_id: Number(poolId),
        ...(filename ? { filename } : {}),
      });
      const result = extractEntries(resp.data.data);
      result.sort((a, b) => a.ts - b.ts);
      if (result.length === 0) {
        setHasMore(false);
        prevScrollHeightRef.current = 0;
      } else {
        setEntries((prev) => [...result, ...prev]);
        setHasMore(result.length === FETCH_LIMIT);
      }
    } catch {
      prevScrollHeightRef.current = 0;
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }

  function handleScroll() {
    const el = scrollRef.current;
    if (!el) return;
    if (el.scrollTop < 120 && hasMore && !loadingMoreRef.current && initialLoaded) {
      void loadMore();
    }
  }

  // ── Search ─────────────────────────────────────────────────────────────────

  const searchQuery = searchText.trim();

  const matchCount = useMemo(() => {
    if (!searchQuery) return 0;
    const esc = searchQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(esc, 'gi');
    return entries.reduce((n, e) => n + (e.content.match(re)?.length ?? 0), 0);
  }, [entries, searchQuery]);

  // ── Download ───────────────────────────────────────────────────────────────

  async function handleStageDownload() {
    if (!machineId || !filename) {
      toast.error("Select a host and log file before downloading.");
      return;
    }
    setStaging(true);
    try {
      const { data } = await logsService.stageDownload(projectKey, Number(machineId), filename);
      setPendingDownload({ token: data.token, fileName: data.file_name, fileSize: data.file_size });
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to prepare logs for download.";
      toast.error(msg);
    } finally {
      setStaging(false);
    }
  }

  async function handleConfirmDownload() {
    if (!pendingDownload) return;
    setFetchingDownload(true);
    try {
      const resp = await logsService.fetchDownload(pendingDownload.token);
      const blob = new Blob([resp.data as BlobPart], { type: "application/zip" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = pendingDownload.fileName;
      a.click();
      URL.revokeObjectURL(url);
      setPendingDownload(null);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to download logs.";
      toast.error(msg);
    } finally {
      setFetchingDownload(false);
    }
  }

  // ── Clear ──────────────────────────────────────────────────────────────────

  async function handleClear() {
    setClearing(true);
    try {
      await logsService.clear(projectKey, appId, {
        ...(machineId ? { machine_id: Number(machineId) } : {}),
        ...(poolId ? { pool_id: Number(poolId) } : {}),
        ...(filename ? { filename } : {}),
      });
      toast.success("Logs cleared. They may take a moment to disappear.");
      setClearConfirmOpen(false);
      setFetchKey((k) => k + 1);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to clear logs.";
      toast.error(msg);
    } finally {
      setClearing(false);
    }
  }

  // ── Render pieces ──────────────────────────────────────────────────────────

  const filterBar = (
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
        disabled={!machineId || !poolOptions.length}
      />

      <FilterSelect
        value={filename}
        onChange={(v) => setFilename(String(v))}
        disabled={!machineId || filesLoading || !fileOptions.length}
        options={[
          { label: filesLoading ? "Loading files…" : !fileOptions.length ? "No files" : "All files", value: "" },
          ...fileOptions,
        ]}
      />

      {/* Search */}
      <div className='relative'>
        <Search className='pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 size-3 text-ink-faint' />
        <input
          type='text'
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          placeholder='Search…'
          className='h-7 pl-6 pr-2.5 rounded-md border border-rim bg-surface text-xs text-ink placeholder:text-ink-faint focus:outline-none focus:border-accent w-36'
        />
        {searchQuery && (
          <span className='pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-[0.6rem] text-ink-faint'>
            {matchCount}
          </span>
        )}
      </div>

      <div className='ml-auto flex items-center gap-1'>
        <Tooltip label={staging ? "Preparing…" : "Download logs"}>
          <button
            type='button'
            onClick={() => void handleStageDownload()}
            disabled={staging || !machineId || !filename}
            className='flex items-center justify-center h-7 w-7 rounded-md border border-rim bg-surface text-ink-faint hover:text-ink transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed'
          >
            <Download className={cn("size-3.5", staging && "animate-pulse")} />
          </button>
        </Tooltip>
        <Tooltip label="Clear logs">
          <button
            type='button'
            onClick={() => setClearConfirmOpen(true)}
            disabled={!machineId}
            className='flex items-center justify-center h-7 w-7 rounded-md border border-rim bg-surface text-ink-faint hover:text-danger transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed'
          >
            <Trash2 className='size-3.5' />
          </button>
        </Tooltip>
        <Tooltip label="Refresh">
          <button
            type='button'
            onClick={() => setFetchKey((k) => k + 1)}
            disabled={!readyToFetch}
            className='flex items-center justify-center h-7 w-7 rounded-md border border-rim bg-surface text-ink-faint hover:text-ink transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed'
          >
            <RefreshCw className={cn("size-3.5", isInitialLoading && "animate-spin")} />
          </button>
        </Tooltip>
        <Tooltip label={expanded ? "Collapse" : "Expand"}>
          <button
            type='button'
            onClick={() => setExpanded((v) => !v)}
            className='flex items-center justify-center h-7 w-7 rounded-md border border-rim bg-surface text-ink-faint hover:text-ink transition-colors cursor-pointer'
          >
            {expanded ? <Minimize2 className='size-3.5' /> : <Maximize2 className='size-3.5' />}
          </button>
        </Tooltip>
      </div>
    </div>
  );

  const logArea = (
    <div
      ref={scrollRef}
      onScroll={handleScroll}
      className='flex-1 overflow-y-auto bg-canvas relative'
      style={{ fontFamily: "ui-monospace, 'SF Mono', Menlo, Monaco, monospace", fontSize: "11px" }}
    >
      {/* Load more indicator at the top */}
      {loadingMore && (
        <div className='sticky top-0 flex items-center justify-center py-2 bg-surface-alt/80 backdrop-blur-sm text-xs text-ink-faint border-b border-rim z-10'>
          Loading logs…
        </div>
      )}
      {!hasMore && initialLoaded && entries.length > 0 && (
        <div className='flex items-center justify-center py-2 text-[0.625rem] text-ink-faint/50'>
          — No more logs to show —
        </div>
      )}

      {isInitialLoading ? (
        <div className='flex items-center justify-center h-full text-ink-faint text-xs'>
          Loading logs…
        </div>
      ) : !readyToFetch ? (
        <div className='flex flex-col items-center justify-center h-full gap-2 text-ink-faint text-xs'>
          <Terminal className='size-5 opacity-30' />
          <span>Select a host and app pool to view logs.</span>
        </div>
      ) : fetchError ? (
        <div className='flex items-center justify-center h-full text-danger text-xs'>
          Failed to load logs.
        </div>
      ) : !entries.length ? (
        <div className='flex flex-col items-center justify-center h-full gap-2 text-ink-faint text-xs'>
          <Terminal className='size-5 opacity-30' />
          <span>No log entries found.</span>
        </div>
      ) : (
        <div className='p-2'>
          {entries.map((entry, i) => {
            const hasMatch = searchQuery
              ? entry.content.toLowerCase().includes(searchQuery.toLowerCase())
              : true;
            return (
              <div
                key={i}
                className={cn(
                  'flex items-baseline gap-1.5 px-2 py-[2px] rounded hover:bg-surface-high/50 leading-relaxed transition-opacity duration-75',
                  searchQuery && !hasMatch ? 'opacity-25' : '',
                )}
              >
                <span className='shrink-0 text-ink-faint w-24'>{fmtLogTime(entry.nanoTs)}</span>
                <span className='text-ink-dim break-all flex-1'>
                  {searchQuery ? highlight(entry.content, searchQuery) : entry.content}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );

  const modals = (
    <>
      <ConfirmModal
        open={clearConfirmOpen}
        title='Clear Logs'
        description='This permanently deletes all log files from disk on the selected host and removes the corresponding entries from the log index. This cannot be undone.'
        confirmLabel='Clear Logs'
        loading={clearing}
        onClose={() => setClearConfirmOpen(false)}
        onConfirm={() => void handleClear()}
      />

      <Modal open={!!pendingDownload} onClose={() => setPendingDownload(null)} title='Download Logs'>
        <div className='space-y-4'>
          <div className='rounded-lg border border-rim bg-surface-alt divide-y divide-rim text-xs'>
            <div className='flex items-center gap-2.5 px-4 py-2.5'>
              <FileArchive className='size-3.5 text-ink-faint shrink-0' />
              <span className='font-mono text-ink truncate'>{pendingDownload?.fileName}</span>
            </div>
            <div className='flex items-center justify-between px-4 py-2.5'>
              <span className='text-ink-faint'>Size</span>
              <span className='font-mono text-ink'>
                {pendingDownload ? formatBytes(pendingDownload.fileSize) : ""}
              </span>
            </div>
          </div>
          <div className='flex justify-end gap-2'>
            <Button
              type='button'
              variant='outline'
              onClick={() => setPendingDownload(null)}
              disabled={fetchingDownload}
            >
              Cancel
            </Button>
            <Button type='button' loading={fetchingDownload} onClick={() => void handleConfirmDownload()}>
              Download
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );

  // ── Expanded modal (portal) ────────────────────────────────────────────────

  if (expanded) {
    return (
      <>
        {/* Collapsed placeholder so the page doesn't reflow */}
        <div className='h-[50vh] rounded-lg border border-rim bg-canvas flex items-center justify-center text-xs text-ink-faint gap-2'>
          <Maximize2 className='size-3.5 opacity-40' />
          <span>Log viewer is open in expanded view</span>
        </div>

        {createPortal(
          <div className='fixed inset-0 z-[9999] flex items-center justify-center p-4'>
            <div className='absolute inset-0 bg-black/80' onClick={() => setExpanded(false)} />
            <div className='relative z-10 w-full h-full max-w-[95vw] max-h-[95vh] rounded-xl border border-rim bg-canvas shadow-2xl flex flex-col overflow-hidden'>
              {/* Modal header with close */}
              <div className='flex items-center justify-between px-4 py-2 border-b border-rim bg-surface-alt shrink-0'>
                <div className='flex items-center gap-2'>
                  <Terminal className='size-3.5 text-ink-faint' />
                  <span className='text-xs font-semibold text-ink'>Logs Explorer</span>
                </div>
                <button
                  type='button'
                  onClick={() => setExpanded(false)}
                  className='cursor-pointer rounded p-0.5 text-ink-faint hover:text-ink hover:bg-surface-high transition-colors'
                >
                  <X className='size-4' />
                </button>
              </div>
              {filterBar}
              {logArea}
            </div>
            {modals}
          </div>,
          document.body,
        )}
      </>
    );
  }

  // ── Normal embedded view ───────────────────────────────────────────────────

  return (
    <>
      <div className='flex flex-col rounded-lg border border-rim overflow-hidden h-[50vh]'>
        {filterBar}
        {logArea}
      </div>
      {modals}
    </>
  );
}
