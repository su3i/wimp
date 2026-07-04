import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Monitor, Layers, Bell, Info, ExternalLink, ShieldAlert } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useProjectStore } from "@/store/project";
import { dashboardService } from "@/services/dashboard.service";
import type { DashboardNotification } from "@/services/dashboard.service";
import { Pagination } from "@/components/ui/Pagination";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";

// ── Helpers ───────────────────────────────────────────────────────────────────

function timeAgo(dateStr: string | null | undefined): string {
  if (!dateStr) return "N/A";
  try {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60_000);
    if (mins < 1) return "Just now";
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  } catch {
    return "N/A";
  }
}

function categoryIcon(cat: string | null | undefined): LucideIcon {
  if (cat === "machine") return Monitor;
  if (cat === "apppool" || cat === "app_pool") return Layers;
  return Bell;
}

function categoryLabel(cat: string | null | undefined): string {
  if (cat === "machine") return "Machine";
  if (cat === "apppool" || cat === "app_pool") return "App Pool";
  if (cat === "iis") return "IIS";
  if (cat === "service") return "Service";
  return cat ?? "System";
}

// ── Level badge ───────────────────────────────────────────────────────────────

const LEVEL_CFG: Record<string, { label: string; cls: string }> = {
  critical: {
    label: "Critical",
    cls: "bg-danger/10 text-danger border border-danger/20",
  },
  warning: {
    label: "Warning",
    cls: "bg-[#d29922]/10 text-[#d29922] border border-[#d29922]/20",
  },
  info: {
    label: "Info",
    cls: "bg-surface-high text-ink-faint border border-rim",
  },
};

function LevelBadge({ level }: { level: string }) {
  const cfg = LEVEL_CFG[level] ?? { label: level, cls: "bg-surface-high text-ink-faint border border-rim" };
  return (
    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-[0.625rem] font-semibold uppercase tracking-wide whitespace-nowrap", cfg.cls)}>
      {cfg.label}
    </span>
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

      <div className='px-4 py-3 min-w-0'>
        <p className='text-xs font-medium text-ink truncate'>{notif.Title ?? ""}</p>
        {notif.Detail ? (
          <p className='mt-0.5 text-[0.6875rem] text-ink-faint truncate'>{notif.Detail}</p>
        ) : null}
      </div>

      <div className='flex items-center gap-1.5 px-4 py-3'>
        <CatIcon className='size-3 text-ink-faint shrink-0' />
        <span className='text-xs text-ink-dim'>{categoryLabel(notif.Category)}</span>
      </div>

      <div className='px-4 py-3 text-right'>
        <span className='text-xs text-ink-faint tabular-nums'>{timeAgo(notif.CreatedAt)}</span>
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
] as const;

type Level = (typeof LEVELS)[number]["value"];
const PAGE_SIZE = 25;

function NoProjectSelected() {
  return (
    <div className='flex flex-col items-center justify-center py-24 text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface'>
        <ShieldAlert className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No project selected</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        Select a project from the sidebar to view its alerts.
      </p>
    </div>
  );
}

export function Alerts() {
  usePageTitle("Alerts");
  const { activeProject } = useProjectStore();
  const [level, setLevel] = useState<Level>("all");
  const [page, setPage] = useState(1);

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
        <h1 className='text-base font-semibold text-ink'>Alerts</h1>
        <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
      </div>

      {/* Alertmanager hint */}
      <div className='flex items-start gap-2.5 rounded-lg border border-rim bg-surface px-4 py-3'>
        <Info className='size-3.5 text-ink-dim shrink-0 mt-0.5' />
        <p className='text-xs text-ink-dim leading-relaxed'>
          Route critical alerts to Slack, PagerDuty, email, or Telegram by configuring an{" "}
          <span className='font-medium text-ink'>Alertmanager receiver</span> in your{" "}
          <span className='font-mono text-[0.6875rem] text-ink'>values.yaml</span>.{" "}
          <a
            href='https://prometheus.io/docs/alerting/latest/configuration/#receiver'
            target='_blank'
            rel='noopener noreferrer'
            className='inline-flex items-center gap-0.5 text-ink hover:text-primary underline underline-offset-2 transition-colors'
          >
            Alertmanager docs <ExternalLink className='size-2.5' />
          </a>
        </p>
      </div>

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
        {/* Body */}
        <div className='min-h-[400px]'>
          {isLoading ? (
            <div className='flex items-center justify-center h-[400px] text-xs text-ink-faint'>
              Loading...
            </div>
          ) : isError ? (
            <div className='flex items-center justify-center gap-2 h-[400px] text-xs text-danger'>
              <AlertTriangle className='size-3.5' /> Failed to load alerts.
            </div>
          ) : notifications.length === 0 ? (
            <div className='flex flex-col items-center justify-center h-[400px] gap-2 text-ink-faint'>
              <Bell className='size-5 opacity-30' />
              <p className='text-xs'>No alerts found.</p>
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
        onPageChange={setPage}
      />
    </div>
  );
}
