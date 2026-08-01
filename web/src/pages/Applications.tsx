import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Layers, AppWindow, Trash2, AlertCircle, Boxes, Plus, Pencil, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { RowMenu } from "@/components/ui/RowMenu";
import { Pagination } from "@/components/ui/Pagination";
import { useProjectStore } from "@/store/project";
import { applicationService } from "@/services/application.service";
import { prometheusService } from "@/services/prometheus.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import type { Application } from "@/types";

// Status/uptime only exist for apps with a health check URL configured - app pool
// state (Started/Stopped) says nothing about whether the app itself is reachable, so
// apps without one show N/A rather than a fabricated status.
function healthCheckStatus(
  app: Application,
  hcStatusMap: Record<number, "up" | "down">,
  hcUptimeMap: Record<number, number>,
) {
  if (!app.HealthCheckURL) return null;
  const raw = hcStatusMap[app.ID];
  const label = raw === "up" ? "Up" : raw === "down" ? "Down" : "No Data";
  const dot = label === "Up" ? "bg-success" : label === "Down" ? "bg-danger animate-pulse" : "bg-ink-faint/40";
  const text = label === "Up" ? "text-success" : label === "Down" ? "text-danger" : "text-ink-faint";
  const ratio = hcUptimeMap[app.ID];
  const uptimePct = ratio != null ? `${(ratio * 100).toFixed(2)}%` : null;
  return { label, dot, text, uptimePct };
}

function StatusCell({ status }: { status: ReturnType<typeof healthCheckStatus> }) {
  if (!status) return <span className='text-xs text-ink-faint'>N/A</span>;
  return (
    <span className={cn("flex items-center gap-1.5 text-xs font-medium", status.text)}>
      {status.label === "Up"
        ? <CheckCircle2 className='size-3 shrink-0' />
        : <span className={cn("size-2 rounded-full shrink-0", status.dot)} />}
      {status.label}
    </span>
  );
}

function TableSkeleton() {
  return (
    <div className='rounded-lg border border-rim overflow-hidden'>
      <div className='border-b border-rim bg-surface-alt px-5 py-2.5 flex gap-8'>
        {["w-32", "w-20", "w-24", "w-28"].map((w, i) => (
          <div key={i} className={cn("h-2.5 rounded bg-surface-high animate-pulse", w)} />
        ))}
      </div>
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className='flex items-center gap-6 px-5 py-4 border-b border-rim last:border-0 animate-pulse'
        >
          <div className='size-6 rounded border border-rim bg-surface-high' />
          <div className='flex-[2] h-3 w-40 rounded bg-surface-high' />
          <div className='flex-1 h-2.5 w-20 rounded bg-surface-high' />
          <div className='flex-1 h-2.5 w-24 rounded bg-surface-high' />
          <div className='flex gap-2'>
            <div className='h-7 w-24 rounded-md bg-surface-high' />
            <div className='h-7 w-20 rounded-md bg-surface-high' />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface-alt'>
        <Boxes className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No applications</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        No applications have been registered for this project yet.
      </p>
    </div>
  );
}

function NoProjectSelected() {
  return (
    <div className='flex flex-col items-center justify-center py-24 text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface'>
        <Layers className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No project selected</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        Select a project from the sidebar to view its applications.
      </p>
    </div>
  );
}

function EditApplicationModal({
  open,
  onClose,
  app,
  projectKey,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
  app: Application;
  projectKey: string;
  onSuccess: () => void;
}) {
  const [name, setName] = useState(app.Name);
  const [hcUrl, setHcUrl] = useState(app.HealthCheckURL ?? "");
  const [hcInterval, setHcInterval] = useState(app.HealthCheckIntervalSeconds ?? 60);
  const [saving, setSaving] = useState(false);

  async function handleSave() {
    if (!name.trim()) return;
    setSaving(true);
    try {
      await applicationService.update(projectKey, app.ID, {
        name: name.trim(),
        health_check_url: hcUrl.trim() || null,
        health_check_interval_seconds: hcUrl.trim() ? hcInterval : 60,
      });
      toast.success("Application updated.");
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to update application.";
      toast.error(msg);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Edit Application">
      <div className="space-y-4">
        <div className="space-y-1.5">
          <label className="text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint">Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            className="w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink placeholder:text-ink-faint focus:outline-none focus:border-accent"
          />
        </div>

        <div className="space-y-1.5">
          <label className="text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint">
            Health Check URL
          </label>
          <input
            type="text"
            value={hcUrl}
            onChange={(e) => setHcUrl(e.target.value)}
            placeholder="https://example.com/health"
            className="w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink font-mono placeholder:text-ink-faint focus:outline-none focus:border-accent"
          />
        </div>

        {hcUrl.trim() && (
          <div className="space-y-1.5">
            <label className="text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint">Health Check Interval</label>
            <select
              value={hcInterval}
              onChange={(e) => setHcInterval(Number(e.target.value))}
              className="w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink focus:outline-none focus:border-accent"
            >
              <option value={30}>30 seconds</option>
              <option value={60}>1 minute</option>
              <option value={120}>2 minutes</option>
              <option value={300}>5 minutes</option>
              <option value={600}>10 minutes</option>
            </select>
          </div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>Cancel</Button>
          <Button type="button" loading={saving} disabled={saving || !name.trim()} onClick={() => void handleSave()}>
            Save
          </Button>
        </div>
      </div>
    </Modal>
  );
}

const TH = "px-5 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";
const PAGE_SIZE = 20;

export function Applications() {
  usePageTitle("Applications");
  const { activeProject } = useProjectStore();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [appName, setAppName] = useState("");
  const [appHcUrl, setAppHcUrl] = useState("");
  const [appHcInterval, setAppHcInterval] = useState(60);
  const [creating, setCreating] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Application | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [editTarget, setEditTarget] = useState<Application | null>(null);

  const {
    data: appsPage,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["applications", activeProject?.Key, page],
    enabled: !!activeProject,
    queryFn: async () => {
      const { data } = await applicationService.list(activeProject!.Key, { page, per_page: PAGE_SIZE });
      return data;
    },
  });

  // Batched Prometheus queries for every app on this page that has a health check URL -
  // same blackbox_http probe_success metric ApplicationDetail.tsx's HealthMonitor reads
  // for a single app, batched here via a regex label match (same trick used elsewhere
  // for machine_id/application_id label sets).
  const hcAppIds = (appsPage?.applications ?? []).filter((a) => !!a.HealthCheckURL).map((a) => a.ID);
  const hcIdKey = hcAppIds.slice().sort((a, b) => a - b).join(",");
  const hcEnabled = prometheusService.isConfigured() && hcAppIds.length > 0;

  const { data: hcStatusResult } = useQuery({
    queryKey: ["applications-hc-status", hcIdKey],
    queryFn: () => prometheusService.instant(`probe_success{job="blackbox_http", application_id=~"${hcIdKey.split(",").join("|")}"}`),
    enabled: hcEnabled,
    refetchInterval: 30_000,
  });
  const { data: hcUptimeResult } = useQuery({
    queryKey: ["applications-hc-uptime", hcIdKey],
    queryFn: () => prometheusService.instant(`avg_over_time(probe_success{job="blackbox_http", application_id=~"${hcIdKey.split(",").join("|")}"}[30d])`),
    enabled: hcEnabled,
    refetchInterval: 60_000,
  });

  const hcStatusMap: Record<number, "up" | "down"> = {};
  for (const r of hcStatusResult ?? []) {
    const id = Number(r.metric.application_id);
    if (!Number.isNaN(id)) hcStatusMap[id] = r.value[1] === "1" ? "up" : "down";
  }
  const hcUptimeMap: Record<number, number> = {};
  for (const r of hcUptimeResult ?? []) {
    const id = Number(r.metric.application_id);
    if (!Number.isNaN(id)) hcUptimeMap[id] = Number(r.value[1]);
  }

  function resetCreateForm() {
    setAppName("");
    setAppHcUrl("");
    setAppHcInterval(60);
  }

  async function handleCreate() {
    if (!appName.trim()) return;
    setCreating(true);
    try {
      await applicationService.create(
        activeProject!.Key,
        appName.trim(),
        appHcUrl.trim() || null,
        appHcUrl.trim() ? appHcInterval : undefined,
      );
      queryClient.invalidateQueries({ queryKey: ["applications", activeProject!.Key] });
      setCreateOpen(false);
      resetCreateForm();
    } catch {
      toast.error("Failed to create application.");
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await applicationService.delete(activeProject!.Key, deleteTarget.ID);
      queryClient.invalidateQueries({ queryKey: ["applications", activeProject!.Key] });
      toast.success(`"${deleteTarget.Name}" deleted.`);
      setDeleteTarget(null);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to delete application.";
      toast.error(msg);
    } finally {
      setDeleting(false);
    }
  }

  if (!activeProject) return <NoProjectSelected />;

  const apps = appsPage?.applications ?? [];
  const totalPages = Math.max(1, Math.ceil((appsPage?.total ?? 0) / PAGE_SIZE));
  const paginated = apps;

  return (
    <div className='space-y-5'>
      {/* Header */}
      <div className='flex items-center justify-between'>
        <div>
          <h1 className='text-base font-semibold text-ink'>Applications</h1>
          <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
          <p className='mt-1 max-w-xl text-[0.6875rem] text-ink-faint'>
            An application is a logical group of app pools across one or more hosts, so you can manage and monitor them together.
          </p>
        </div>
        <Button size='sm' onClick={() => setCreateOpen(true)}>
          <Plus className='size-3.5' />
          New Application
        </Button>
      </div>

      {/* Content */}
      {isLoading ? (
        <TableSkeleton />
      ) : isError ? (
        <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
          <AlertCircle className='size-4 shrink-0' />
          Failed to load applications. Check your connection and try again.
        </div>
      ) : !apps?.length ? (
        <EmptyState />
      ) : (
        // Header and rows are all direct children of this single grid (row wrappers use
        // `contents` so they don't start their own nested grid) - that keeps every
        // column's width consistent across the header and every row. Separate grids that
        // merely share the same grid-cols classes each size their own tracks
        // independently, so a long value (a URL, a long app name) in one row can widen
        // that row's columns without widening the header's - which is what was skewed.
        <div className='grid grid-cols-[2fr_0.7fr_1.5fr_0.7fr_0.8fr_auto] rounded-lg border border-rim overflow-hidden'>
          <div className={cn(TH, "border-b border-rim bg-surface-alt")}>Application Name</div>
          <div className={cn(TH, "border-b border-rim bg-surface-alt")}>Instances</div>
          <div className={cn(TH, "border-b border-rim bg-surface-alt")}>URL</div>
          <div className={cn(TH, "border-b border-rim bg-surface-alt")}>Status</div>
          <div className={cn(TH, "border-b border-rim bg-surface-alt")}>Uptime</div>
          <div className='border-b border-rim bg-surface-alt px-4 py-2.5' />

          {/* Rows */}
          {paginated.map((app, idx) => {
            const total = app.pool_total ?? 0;
            const status = healthCheckStatus(app, hcStatusMap, hcUptimeMap);
            const isLast = idx === paginated.length - 1;
            const cellBorder = !isLast && "border-b border-rim";
            return (
              <div
                key={app.ID}
                className='contents group cursor-pointer'
                onClick={() => navigate(`/applications/${app.ID}`)}
              >
                {/* Name */}
                <div className={cn("flex min-w-0 items-center gap-3 px-5 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                  <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                    <AppWindow className='size-3 text-ink-faint' />
                  </div>
                  <span className='font-mono text-xs text-ink truncate'>{app.Name}</span>
                </div>

                {/* Instances */}
                <div className={cn("px-5 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                  <span className='text-xs text-ink-dim'>
                    {total} {total === 1 ? "instance" : "instances"}
                  </span>
                </div>

                {/* URL */}
                <div className={cn("min-w-0 px-5 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)} title={app.HealthCheckURL ?? undefined}>
                  {app.HealthCheckURL ? (
                    <span className='block truncate font-mono text-xs text-ink-dim'>{app.HealthCheckURL}</span>
                  ) : (
                    <span className='text-xs text-ink-faint'>N/A</span>
                  )}
                </div>

                {/* Status */}
                <div className={cn("px-5 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                  <StatusCell status={status} />
                </div>

                {/* Uptime */}
                <div className={cn("px-5 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                  <span className='text-xs text-ink-dim'>{status?.uptimePct ?? "N/A"}</span>
                </div>

                {/* Menu */}
                <div className={cn("flex items-center justify-center px-2 py-3 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                  <RowMenu
                    items={[
                      {
                        icon: Pencil,
                        label: "Edit",
                        onClick: () => setEditTarget(app),
                      },
                      { type: "separator" },
                      {
                        icon: Trash2,
                        label: "Delete",
                        variant: "danger",
                        onClick: () => setDeleteTarget(app),
                      },
                    ]}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}

      <Pagination
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />

      <ConfirmModal
        open={!!deleteTarget}
        title='Delete Application'
        description={`Are you sure you want to delete "${deleteTarget?.Name}"? This cannot be undone.`}
        confirmLabel='Delete'
        loading={deleting}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void handleDelete()}
      />

      <Modal
        open={createOpen}
        onClose={() => { setCreateOpen(false); resetCreateForm(); }}
        title='New Application'
      >
        <form
          onSubmit={(e) => { e.preventDefault(); void handleCreate(); }}
          className='space-y-4'
        >
          <div className='space-y-1.5'>
            <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>Name</label>
            <Input
              placeholder='e.g. My Web App'
              value={appName}
              onChange={(e) => setAppName(e.target.value)}
              autoFocus
            />
          </div>

          <div className='space-y-1.5'>
            <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Health Check URL
            </label>
            <Input
              placeholder='https://example.com/health'
              value={appHcUrl}
              onChange={(e) => setAppHcUrl(e.target.value)}
            />
          </div>

          {appHcUrl.trim() && (
            <div className='space-y-1.5'>
              <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>Health Check Interval</label>
              <select
                value={appHcInterval}
                onChange={(e) => setAppHcInterval(Number(e.target.value))}
                className='w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink focus:outline-none focus:border-accent'
              >
                <option value={30}>30 seconds</option>
                <option value={60}>1 minute</option>
                <option value={120}>2 minutes</option>
                <option value={300}>5 minutes</option>
                <option value={600}>10 minutes</option>
              </select>
            </div>
          )}

          <div className='flex justify-end gap-2'>
            <Button type='button' variant='outline' onClick={() => { setCreateOpen(false); resetCreateForm(); }} disabled={creating}>
              Cancel
            </Button>
            <Button type='submit' loading={creating} disabled={!appName.trim()}>
              Create
            </Button>
          </div>
        </form>
      </Modal>

      {editTarget && activeProject && (
        <EditApplicationModal
          open={!!editTarget}
          onClose={() => setEditTarget(null)}
          app={editTarget}
          projectKey={activeProject.Key}
          onSuccess={() => queryClient.invalidateQueries({ queryKey: ["applications", activeProject.Key] })}
        />
      )}
    </div>
  );
}
