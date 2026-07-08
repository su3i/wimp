import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Bell, Info, ExternalLink, ShieldAlert, X } from "lucide-react";
import { useProjectStore } from "@/store/project";
import { dashboardService } from "@/services/dashboard.service";
import type { DashboardNotification } from "@/services/dashboard.service";
import { Pagination } from "@/components/ui/Pagination";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";
import { categoryIcon, categoryLabel, levelConfig } from "@/utils/notifications";
import { timeAgo } from "@/utils/time";

// ── Level badge ───────────────────────────────────────────────────────────────

function LevelBadge({ level }: { level: string }) {
  const cfg = levelConfig(level);
  return (
    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-[0.625rem] font-semibold uppercase tracking-wide whitespace-nowrap", cfg.cls)}>
      {cfg.label}
    </span>
  );
}

// ── Table header ──────────────────────────────────────────────────────────────

const TH = "px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";

function TableHeader() {
  return (
    <div className='grid grid-cols-[84px_1fr_124px_92px] border-b border-rim bg-surface-alt'>
      <div className={TH}>Level</div>
      <div className={TH}>Message</div>
      <div className={TH}>Category</div>
      <div className={cn(TH, "text-right")}>Time</div>
    </div>
  );
}

// ── Skeleton ──────────────────────────────────────────────────────────────────

function RowSkeleton() {
  return (
    <div className='grid grid-cols-[84px_1fr_124px_92px] items-center border-b border-rim last:border-0 animate-pulse'>
      <div className='px-4 py-3'>
        <div className='h-4 w-12 rounded bg-surface-high' />
      </div>
      <div className='px-4 py-3'>
        <div className='h-2.5 w-2/3 rounded bg-surface-high' />
      </div>
      <div className='px-4 py-3'>
        <div className='h-2.5 w-16 rounded bg-surface-high' />
      </div>
      <div className='px-4 py-3 flex justify-end'>
        <div className='h-2.5 w-12 rounded bg-surface-high' />
      </div>
    </div>
  );
}

// ── Table row ─────────────────────────────────────────────────────────────────

function AlertRow({ notif }: { notif: DashboardNotification }) {
  const CatIcon = categoryIcon(notif.Category);
  return (
    <div className='grid grid-cols-[84px_1fr_124px_92px] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'>
      <div className='px-4 py-3'>
        <LevelBadge level={notif.Level ?? "info"} />
      </div>

      <div className='px-4 py-3 min-w-0 flex items-center gap-1.5 overflow-hidden'>
        <span className='text-xs font-medium text-ink shrink-0 truncate'>{notif.Title ?? ""}</span>
        {notif.Detail && (
          <>
            <span className='text-ink-faint/40 text-sm shrink-0'>/</span>
            <span className='text-xs text-ink-dim truncate'>{notif.Detail.toLowerCase()}</span>
          </>
        )}
      </div>

      <div className='flex items-center gap-1.5 px-4 py-3'>
        <CatIcon className='size-3 text-ink-faint shrink-0' />
        <span className='text-xs text-ink-dim'>{categoryLabel(notif.Category)}</span>
      </div>

      <div className='px-4 py-3 text-right'>
        <span className='text-xs text-ink-faint tabular-nums'>{timeAgo(notif.CreatedAt, "N/A")}</span>
      </div>
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

const LEVELS = [
  { value: "all", label: "All" },
  { value: "info", label: "Info" },
  { value: "warning", label: "Warning" },
  { value: "critical", label: "Critical" },
  { value: "sev", label: "Sev" },
] as const;

type Level = (typeof LEVELS)[number]["value"];
const PAGE_SIZE = 10;

function NoProjectSelected() {
  return (
    <div className='flex flex-col items-center justify-center py-24 text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface'>
        <ShieldAlert className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No project selected</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        Select a project from the sidebar to view its activity.
      </p>
    </div>
  );
}

export function Activity() {
  usePageTitle("Activity");
  const { activeProject } = useProjectStore();
  const [level, setLevel] = useState<Level>("all");
  const [page, setPage] = useState(1);
  const [showHint, setShowHint] = useState(true);

  const { data, isLoading, isError } = useQuery({
    queryKey: ["notifications-full", activeProject?.Key, level, page],
    enabled: !!activeProject,
    queryFn: () =>
      dashboardService.listNotifications(activeProject!.Key, {
        page,
        limit: PAGE_SIZE,
        ...(level !== "all" ? { level } : {}),
      }),
  });

  const notifications = data?.notifications ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function changeLevel(l: Level) {
    setLevel(l);
    setPage(1);
  }

  if (!activeProject) return <NoProjectSelected />;

  return (
    <div className='space-y-4'>
      {/* Header */}
      <div>
        <h1 className='text-base font-semibold text-ink'>Activity</h1>
        <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
        <p className='mt-1 max-w-xl text-[0.6875rem] text-ink-faint'>
          A live feed of host connects, app pool and site changes, and alerts across this project, newest first.
        </p>
      </div>

      {/* Alertmanager hint */}
      {showHint && (
        <div className='flex items-start gap-2.5 rounded-lg border border-rim bg-surface px-4 py-3'>
          <Info className='size-3.5 text-ink-dim shrink-0 mt-0.5' />
          <p className='text-xs text-ink-dim leading-relaxed flex-1'>
            Get notified on critical activity via Slack, PagerDuty, email, or Telegram by configuring an{" "}
            <a
              href='https://prometheus.io/docs/alerting/latest/configuration/#receiver'
              target='_blank'
              rel='noopener noreferrer'
              className='inline-flex items-center gap-0.5 text-ink hover:text-primary underline underline-offset-2 transition-colors'
            >
              Alertmanager receiver <ExternalLink className='size-2.5' />
            </a>{" "}in your deployment settings.
          </p>
          <button
            type='button'
            onClick={() => setShowHint(false)}
            className='cursor-pointer shrink-0 flex items-center justify-center size-6 rounded-md border-[1.5px] border-rim text-ink-faint hover:text-ink hover:border-ink-faint transition-colors'
          >
            <X className='size-3' />
          </button>
        </div>
      )}

      {/* Filter tabs */}
      <div className='flex items-center gap-1'>
        {LEVELS.map((l) => (
          <button
            key={l.value}
            type='button'
            onClick={() => changeLevel(l.value)}
            className={cn(
              "cursor-pointer h-7 px-3 rounded-md text-xs font-medium transition-colors",
              level === l.value
                ? "bg-primary/10 text-primary"
                : "text-ink-faint hover:text-ink hover:bg-surface-high",
            )}
          >
            {l.label}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className='rounded-lg border border-rim overflow-hidden'>
        <TableHeader />
        {/* Body */}
        <div className='min-h-[400px]'>
          {isLoading ? (
            Array.from({ length: PAGE_SIZE }).map((_, i) => <RowSkeleton key={i} />)
          ) : isError ? (
            <div className='flex items-center justify-center gap-2 h-[400px] text-xs text-danger'>
              <AlertTriangle className='size-3.5' /> Failed to load activity.
            </div>
          ) : notifications.length === 0 ? (
            <div className='flex flex-col items-center justify-center h-[400px] gap-2 text-ink-faint'>
              <Bell className='size-5 opacity-30' />
              <p className='text-xs'>No activity found.</p>
            </div>
          ) : (
            notifications.map((n) => <AlertRow key={n.ID} notif={n} />)
          )}
        </div>
      </div>

      {/* Pagination */}
      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
      />
    </div>
  );
}
