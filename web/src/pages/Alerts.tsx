import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Monitor, Layers, Bell, Info, ExternalLink } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { api } from "@/lib/axios";
import { usePageTitle } from "@/utils/usePageTitle";
import { cn } from "@/utils/cn";
import type { DashboardNotification } from "@/services/dashboard.service";

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

const NOTIF_DOT: Record<string, string> = {
  error: "bg-danger",
  warning: "bg-[#d29922]",
  info: "bg-ink-faint",
};

function categoryIcon(cat: string | null | undefined): LucideIcon {
  if (cat === "machine") return Monitor;
  if (cat === "app_pool") return Layers;
  return Bell;
}

// ── Row (mirrors Dashboard NotifRow) ─────────────────────────────────────────

function NotifRow({ notif }: { notif: DashboardNotification }) {
  const Icon = categoryIcon(notif.Category);
  return (
    <div className='flex items-center gap-2.5 py-2 border-b border-rim last:border-0'>
      <span
        className={cn("size-1.5 rounded-full shrink-0", NOTIF_DOT[notif.Level ?? ""] ?? "bg-ink-faint")}
      />
      <Icon className='size-3.5 text-ink-faint shrink-0' />
      <div className='flex-1 min-w-0'>
        <p className='text-xs text-ink truncate'>{notif.Title ?? ""}</p>
        {notif.Detail ? <p className='text-[0.6875rem] text-ink-faint truncate'>{notif.Detail}</p> : null}
      </div>
      <span className='text-xs text-ink-faint shrink-0 w-14 text-right'>{timeAgo(notif.CreatedAt)}</span>
    </div>
  );
}

// ── Filter pill ───────────────────────────────────────────────────────────────

function Pill({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type='button'
      onClick={onClick}
      className={cn(
        "cursor-pointer h-7 px-3 rounded-md text-xs font-medium transition-colors",
        active
          ? "bg-primary/10 text-primary border border-primary/20"
          : "bg-surface border border-rim text-ink-faint hover:text-ink",
      )}
    >
      {children}
    </button>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

const LEVELS = [
  { value: "all", label: "All" },
  { value: "info", label: "Info" },
  { value: "warning", label: "Warning" },
  { value: "error", label: "Critical" },
] as const;

type Level = (typeof LEVELS)[number]["value"];
const PAGE_SIZE = 20;

export function Alerts() {
  usePageTitle("Alerts");
  const [level, setLevel] = useState<Level>("all");
  const [page, setPage] = useState(1);

  const { data, isLoading, isError } = useQuery({
    queryKey: ["notifications-full", level, page],
    queryFn: async () => {
      const params: Record<string, string | number> = { page, limit: PAGE_SIZE };
      if (level !== "all") params.level = level;
      const { data } = await api.get("/notifications", { params });
      return data as { notifications: DashboardNotification[]; total: number };
    },
  });

  const notifications = data?.notifications ?? [];
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE));

  function changeLevel(l: Level) {
    setLevel(l);
    setPage(1);
  }

  return (
    <div className='space-y-4'>
      {/* Header */}
      <div>
        <h1 className='text-xl font-semibold text-ink tracking-tight'>Alerts</h1>
        <p className='mt-1 text-sm text-ink-faint'>
          All system notifications and alerts across your project.
        </p>
      </div>

      {/* Alertmanager hint */}
      <div className='flex items-start gap-2.5 rounded-lg border border-rim bg-surface px-4 py-3'>
        <Info className='size-3.5 text-ink-dim shrink-0 mt-0.5' />
        <p className='text-xs text-ink-dim leading-relaxed'>
          Want critical alerts delivered to Slack, PagerDuty, email, or Telegram? Configure an{" "}
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

      {/* Filters */}
      <div className='flex items-center gap-2'>
        {LEVELS.map((l) => (
          <Pill key={l.value} active={level === l.value} onClick={() => changeLevel(l.value)}>
            {l.label}
          </Pill>
        ))}
      </div>

      {/* List - same card style as Dashboard Recent Activity */}
      <div className='rounded-lg border border-rim bg-surface px-4 pt-4 pb-2'>
        {isLoading ? (
          <div className='flex items-center justify-center py-12 text-xs text-ink-faint'>Loading…</div>
        ) : isError ? (
          <div className='flex items-center justify-center gap-2 py-12 text-xs text-danger'>
            <AlertTriangle className='size-3.5' /> Failed to load alerts.
          </div>
        ) : notifications.length === 0 ? (
          <div className='flex flex-col items-center justify-center py-12 gap-2 text-ink-faint'>
            <Bell className='size-5 opacity-30' />
            <p className='text-xs'>No alerts found.</p>
          </div>
        ) : (
          <div className='overflow-y-auto pr-1'>
            {notifications.map((n) => (
              <NotifRow key={n.ID} notif={n} />
            ))}
          </div>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className='flex items-center justify-end gap-1 text-xs text-ink-faint'>
          <button
            type='button'
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className='cursor-pointer h-7 px-3 rounded-md border border-rim bg-surface hover:text-ink transition-colors disabled:opacity-40 disabled:cursor-not-allowed'
          >
            Previous
          </button>
          <span className='px-3'>
            {page} / {totalPages}
          </span>
          <button
            type='button'
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
            className='cursor-pointer h-7 px-3 rounded-md border border-rim bg-surface hover:text-ink transition-colors disabled:opacity-40 disabled:cursor-not-allowed'
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
