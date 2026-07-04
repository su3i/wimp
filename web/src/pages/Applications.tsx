import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Layers, AppWindow, RotateCw, RefreshCw, Trash2, AlertCircle, Boxes, Plus } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { RowMenu } from "@/components/ui/RowMenu";
import { Pagination } from "@/components/ui/Pagination";
import { useProjectStore } from "@/store/project";
import { applicationService } from "@/services/application.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import type { ApplicationDetail } from "@/types";

// ── Health helpers ────────────────────────────────────────────────────────────

function poolHealth(app: ApplicationDetail) {
  const pools = app.app_pools ?? [];
  const healthy = pools.filter((p) => p.State === "Started").length;
  return { healthy, total: pools.length };
}

function HealthStatus({ healthy, total }: { healthy: number; total: number }) {
  const dot =
    total === 0
      ? "bg-ink-faint"
      : healthy === total
      ? "bg-success"
      : healthy === 0
      ? "bg-danger"
      : "bg-warning";

  const text =
    total === 0
      ? "text-ink-faint"
      : healthy === total
      ? "text-success"
      : healthy === 0
      ? "text-danger"
      : "text-warning";

  return (
    <div className='flex items-center gap-2'>
      <span className={cn("size-1.5 rounded-full shrink-0", dot)} />
      <span className={cn("text-xs", text)}>
        {healthy}/{total} Healthy
      </span>
    </div>
  );
}

// ── Skeleton ──────────────────────────────────────────────────────────────────

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

// ── Empty / error states ──────────────────────────────────────────────────────

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

// ── Page ──────────────────────────────────────────────────────────────────────

const TH = "px-5 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";
const PAGE_SIZE = 20;

type ActionState = Record<number, "restarting" | "recycling">;

export function Applications() {
  usePageTitle("Applications");
  const { activeProject } = useProjectStore();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [acting, setActing] = useState<ActionState>({});
  const [createOpen, setCreateOpen] = useState(false);
  const [appName, setAppName] = useState("");
  const [creating, setCreating] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ApplicationDetail | null>(null);
  const [deleting, setDeleting] = useState(false);

  const {
    data: apps,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["applications", activeProject?.Key],
    enabled: !!activeProject,
    queryFn: async () => {
      const { data: listData } = await applicationService.list(activeProject!.Key);
      const list = listData.applications ?? [];
      if (!list.length) return [];
      // Hydrate with pool detail in parallel
      return Promise.all(
        list.map((app) => applicationService.get(activeProject!.Key, app.ID).then((r) => r.data.application))
      );
    },
  });

  async function handleCreate() {
    if (!appName.trim()) return;
    setCreating(true);
    try {
      await applicationService.create(activeProject!.Key, appName.trim());
      queryClient.invalidateQueries({ queryKey: ["applications", activeProject!.Key] });
      setCreateOpen(false);
      setAppName("");
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

  async function runCommand(app: ApplicationDetail, cmd: "restart" | "recycle") {
    const pools = app.app_pools ?? [];
    if (!pools.length) return;
    setActing((prev) => ({ ...prev, [app.ID]: cmd === "restart" ? "restarting" : "recycling" }));
    try {
      await Promise.all(
        pools.map((pool) => applicationService.poolCommand(activeProject!.Key, pool.machine.ID, pool.ID, cmd))
      );
      queryClient.invalidateQueries({ queryKey: ["applications", activeProject!.Key] });
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Command failed. Please try again.";
      toast.error(msg, { description: `${app.Name} - ${cmd}` });
    } finally {
      setActing((prev) => {
        const next = { ...prev };
        delete next[app.ID];
        return next;
      });
    }
  }

  if (!activeProject) return <NoProjectSelected />;

  const appTotal = apps?.length ?? 0;
  const totalPages = Math.max(1, Math.ceil(appTotal / PAGE_SIZE));
  const paginated = apps?.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE) ?? [];

  return (
    <div className='space-y-5'>
      {/* Header */}
      <div className='flex items-center justify-between'>
        <div>
          <h1 className='text-base font-semibold text-ink'>Applications</h1>
          <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
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
        <div className='rounded-lg border border-rim overflow-hidden'>
          {/* Column headers */}
          <div className='grid grid-cols-[2fr_1fr_1fr_auto] border-b border-rim bg-surface-alt'>
            <div className={TH}>Application Name</div>
            <div className={TH}>Instances</div>
            <div className={TH}>Health Status</div>
            <div className='px-4 py-2.5' />
          </div>

          {/* Rows */}
          {paginated.map((app) => {
            const { healthy, total } = poolHealth(app);
            const busy = acting[app.ID];
            return (
              <div
                key={app.ID}
                className='grid grid-cols-[2fr_1fr_1fr_auto] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100 cursor-pointer'
                onClick={() => navigate(`/applications/${app.ID}`)}
              >
                {/* Name */}
                <div className='flex items-center gap-3 px-5 py-3.5'>
                  <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                    <AppWindow className='size-3 text-ink-faint' />
                  </div>
                  <span className='font-mono text-xs text-ink'>{app.Name}</span>
                </div>

                {/* Instances */}
                <div className='px-5 py-3.5'>
                  <span className='text-xs text-ink-dim'>
                    {total} {total === 1 ? "instance" : "instances"}
                  </span>
                </div>

                {/* Health */}
                <div className='px-5 py-3.5'>
                  <HealthStatus healthy={healthy} total={total} />
                </div>

                {/* Menu */}
                <div className='flex items-center justify-center px-2 py-3'>
                  <RowMenu
                    disabled={!!busy}
                    items={[
                      {
                        icon: RotateCw,
                        label: "Restart All",
                        disabled: total === 0,
                        onClick: () => void runCommand(app, "restart"),
                      },
                      {
                        icon: RefreshCw,
                        label: "Recycle All",
                        disabled: total === 0,
                        onClick: () => void runCommand(app, "recycle"),
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
        onClose={() => {
          setCreateOpen(false);
          setAppName("");
        }}
        title='New Application'
      >
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void handleCreate();
          }}
          className='space-y-4'
        >
          <Input
            placeholder='Application name'
            value={appName}
            onChange={(e) => setAppName(e.target.value)}
            autoFocus
          />
          <div className='flex justify-end gap-2'>
            <Button
              type='button'
              variant='outline'
              onClick={() => {
                setCreateOpen(false);
                setAppName("");
              }}
              disabled={creating}
            >
              Cancel
            </Button>
            <Button type='submit' loading={creating} disabled={!appName.trim()}>
              Proceed
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
