import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ArrowLeft,
  AppWindow,
  Plus,
  Server,
  ChevronRight,
  ChevronDown,
  Layers,
  AlertCircle,
  Check,
  Cpu,
  Play,
  Square,
  RotateCw,
  RefreshCw,
  Pencil,
  Trash2,
} from "lucide-react";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { RowMenu } from "@/components/ui/RowMenu";
import type { RowMenuItem } from "@/components/ui/RowMenu";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { useProjectStore } from "@/store/project";
import { LogsViewer } from "@/components/application/LogsViewer";
import { applicationService } from "@/services/application.service";
import { appPoolService } from "@/services/appPool.service";
import { machineService } from "@/services/machine.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import type { AppPoolWithDetails, MachineWithPools } from "@/types";

// ── Shared helpers (mirror AppPoolsTab) ───────────────────────────────────────

const TH = "px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";

const poolStatusCfg: Record<string, { dot: string; text: string }> = {
  Started: { dot: "bg-success", text: "text-success" },
  Stopped: { dot: "bg-danger", text: "text-danger" },
  Starting: { dot: "bg-warning", text: "text-warning" },
  Stopping: { dot: "bg-warning", text: "text-warning" },
};

function PoolStatus({ state }: { state: string }) {
  const cfg = poolStatusCfg[state] ?? { dot: "bg-ink-faint", text: "text-ink-faint" };
  return (
    <div className='flex items-center gap-2'>
      <span className={cn("size-1.5 rounded-full shrink-0", cfg.dot)} />
      <span className={cn("text-xs", cfg.text)}>{state}</span>
    </div>
  );
}

// ── Empty state ───────────────────────────────────────────────────────────────

function EmptyPools({ onAdd }: { onAdd: () => void }) {
  return (
    <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface-alt'>
        <Layers className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No app pools assigned</p>
      <p className='mt-1 mb-5 max-w-xs text-xs text-ink-faint'>
        This application has no app pools yet. Assign pools from your machines to start managing them here.
      </p>
      <Button size='sm' onClick={onAdd}>
        <Plus className='size-3.5' />
        Add App Pools
      </Button>
    </div>
  );
}

// ── Edit Log Path Modal ───────────────────────────────────────────────────────

function EditLogPathModal({
  open,
  onClose,
  pool,
  appName,
  projectKey,
  appId,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
  pool: AppPoolWithDetails;
  appName: string;
  projectKey: string;
  appId: number;
  onSuccess: () => void;
}) {
  const [logPath, setLogPath] = useState(pool.log_path ?? "");
  const [saving, setSaving] = useState(false);

  // Reset when pool changes
  useState(() => {
    setLogPath(pool.log_path ?? "");
  });

  async function handleSave() {
    setSaving(true);
    try {
      await applicationService.updatePoolLogPath(projectKey, appId, pool.ID, logPath);
      toast.success("Log path updated.");
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to update log path.";
      toast.error(msg);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title='Edit Log Path'>
      <div className='space-y-4'>
        {/* Read-only context */}
        <div className='rounded-lg border border-rim bg-surface-alt divide-y divide-rim text-xs'>
          <div className='flex items-center justify-between px-4 py-2.5'>
            <span className='text-ink-faint'>Application</span>
            <span className='font-mono text-ink'>{appName}</span>
          </div>
          <div className='flex items-center justify-between px-4 py-2.5'>
            <span className='text-ink-faint'>App Pool</span>
            <span className='font-mono text-ink'>{pool.Name}</span>
          </div>
          <div className='flex items-center justify-between px-4 py-2.5'>
            <span className='text-ink-faint'>Machine</span>
            <span className='font-mono text-ink'>{pool.machine?.Hostname?.toLowerCase() ?? "N/A"}</span>
          </div>
          <div className='flex items-center justify-between px-4 py-2.5'>
            <span className='text-ink-faint'>Status</span>
            <PoolStatus state={pool.State} />
          </div>
        </div>

        {/* Editable field */}
        <div className='space-y-1.5'>
          <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
            Log Path
          </label>
          <input
            type='text'
            value={logPath}
            onChange={(e) => setLogPath(e.target.value)}
            placeholder='e.g. C:\inetpub\logs\LogFiles\W3SVC1'
            autoFocus
            className='w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink font-mono placeholder:text-ink-faint focus:outline-none focus:border-accent'
          />
          <p className='text-[0.625rem] text-ink-faint'>
            Windows path to the IIS log directory for this app pool.
          </p>
        </div>

        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button
            type='button'
            loading={saving}
            disabled={saving || !logPath.trim()}
            onClick={() => void handleSave()}
          >
            Save
          </Button>
        </div>
      </div>
    </Modal>
  );
}

// ── Pool list ─────────────────────────────────────────────────────────────────

function PoolList({
  pools,
  projectKey,
  appId,
  appName,
  queryKey,
}: {
  pools: AppPoolWithDetails[];
  projectKey: string;
  appId: number;
  appName: string;
  queryKey: unknown[];
}) {
  const queryClient = useQueryClient();
  const [acting, setActing] = useState<Record<number, string>>({});
  const [editingPool, setEditingPool] = useState<AppPoolWithDetails | null>(null);
  const [removeTarget, setRemoveTarget] = useState<AppPoolWithDetails | null>(null);
  const [removing, setRemoving] = useState(false);
  async function runCmd(pool: AppPoolWithDetails, cmd: "start" | "stop" | "restart" | "recycle") {
    setActing((prev) => ({ ...prev, [pool.ID]: cmd }));
    try {
      await appPoolService.command(projectKey, pool.machine.ID, pool.ID, cmd);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Command failed. Please try again.";
      toast.error(msg, { description: `${pool.Name} - ${cmd}` });
    } finally {
      setActing((prev) => {
        const n = { ...prev };
        delete n[pool.ID];
        return n;
      });
    }
  }

  async function handleRemove() {
    if (!removeTarget) return;
    setRemoving(true);
    try {
      await applicationService.removePool(projectKey, appId, removeTarget.ID);
      queryClient.invalidateQueries({ queryKey });
      toast.success(`"${removeTarget.Name}" removed from application.`);
      setRemoveTarget(null);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to remove app pool.";
      toast.error(msg);
    } finally {
      setRemoving(false);
    }
  }

  const cols = "grid-cols-[2fr_1fr_1fr_auto]";

  return (
    <div className='overflow-hidden'>
      <div className={`grid ${cols} border-b border-rim bg-surface-alt sticky top-0 z-10`}>
        <div className={TH}>Name</div>
        <div className={TH}>Machine</div>
        <div className={TH}>Status</div>
        <div className='px-4 py-2.5' />
      </div>

      {pools.map((pool) => {
        const busy = acting[pool.ID];
        const started = pool.State === "Started";
        const menuItems: RowMenuItem[] = [
          { icon: Pencil, label: "Edit Log Path", onClick: () => setEditingPool(pool) },
          { type: "separator" },
          ...(started
            ? [
                {
                  icon: Square,
                  label: "Stop",
                  variant: "danger" as const,
                  onClick: () => runCmd(pool, "stop"),
                },
                { icon: RotateCw, label: "Restart", onClick: () => runCmd(pool, "restart") },
                { icon: RefreshCw, label: "Recycle", onClick: () => runCmd(pool, "recycle") },
              ]
            : [{ icon: Play, label: "Start", onClick: () => runCmd(pool, "start") }]),
          { type: "separator" },
          { icon: Trash2, label: "Remove", variant: "danger", onClick: () => setRemoveTarget(pool) },
        ];
        return (
          <div
            key={pool.ID}
            className={`grid ${cols} items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100`}
          >
            <div className='flex items-center gap-3 px-4 py-3.5'>
              <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                <Cpu className='size-3 text-ink-faint' />
              </div>
              <span className='font-mono text-xs text-ink truncate'>{pool.Name}</span>
            </div>

            <div className='px-4 py-3.5'>
              <span className='font-mono text-xs text-ink-dim'>
                {pool.machine?.Hostname?.toLowerCase() ?? "N/A"}
              </span>
            </div>

            <div className='px-4 py-3.5'>
              <PoolStatus state={pool.State} />
            </div>

            <div className='flex items-center justify-center px-2 py-3'>
              <RowMenu items={menuItems} disabled={!!busy} />
            </div>
          </div>
        );
      })}

      {editingPool && (
        <EditLogPathModal
          open={!!editingPool}
          onClose={() => setEditingPool(null)}
          pool={editingPool}
          appName={appName}
          projectKey={projectKey}
          appId={appId}
          onSuccess={() => queryClient.invalidateQueries({ queryKey })}
        />
      )}

      <ConfirmModal
        open={!!removeTarget}
        title='Remove App Pool'
        description={`Remove "${removeTarget?.Name}" from this application? The pool itself will not be stopped or deleted.`}
        confirmLabel='Remove'
        loading={removing}
        onClose={() => setRemoveTarget(null)}
        onConfirm={() => void handleRemove()}
      />
    </div>
  );
}

// ── Add App Pool Modal ────────────────────────────────────────────────────────

interface AddAppPoolModalProps {
  open: boolean;
  onClose: () => void;
  projectKey: string;
  appId: number;
  assignedPoolIds: Set<number>;
  onSuccess: () => void;
}

function AddAppPoolModal({
  open,
  onClose,
  projectKey,
  appId,
  assignedPoolIds,
  onSuccess,
}: AddAppPoolModalProps) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [submitting, setSubmitting] = useState(false);

  const { data: machines, isLoading } = useQuery({
    queryKey: ["machines-with-pools", projectKey],
    enabled: open,
    queryFn: async () => {
      const { data } = await machineService.list(projectKey);
      return data.machines ?? [];
    },
  });

  function toggleExpand(machineId: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      next.has(machineId) ? next.delete(machineId) : next.add(machineId);
      return next;
    });
  }

  function togglePool(poolId: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(poolId) ? next.delete(poolId) : next.add(poolId);
      return next;
    });
  }

  async function handleSubmit() {
    if (selected.size === 0) {
      onClose();
      return;
    }

    setSubmitting(true);
    try {
      await applicationService.syncPools(projectKey, appId, [...selected]);
      toast.success(`${selected.size} pool${selected.size !== 1 ? "s" : ""} added.`);
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to add app pools. Please try again.";
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  }

  const hasChanges = selected.size > 0;

  return (
    <Modal open={open} onClose={onClose} title='Add App Pools'>
      <div className='space-y-4'>
        {isLoading ? (
          <div className='space-y-2'>
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className='h-10 rounded-md bg-surface-high animate-pulse' />
            ))}
          </div>
        ) : !machines?.some((m) => (m.app_pools ?? []).some((p) => p.State === "Started")) ? (
          <p className='text-xs text-ink-faint text-center py-6'>
            No running app pools found on any machine.
          </p>
        ) : (
          <div className='rounded-lg border border-rim overflow-hidden max-h-[400px] overflow-y-auto'>
            {machines.map((machine: MachineWithPools) => {
              const pools = (machine.app_pools ?? []).filter((p) => p.State === "Started");
              if (pools.length === 0) return null;
              const isOpen = expanded.has(machine.ID);
              const availableCount = pools.filter((p) => !assignedPoolIds.has(p.ID)).length;
              return (
                <div key={machine.ID} className='border-b border-rim last:border-0'>
                  <button
                    type='button'
                    className='w-full flex items-center gap-3 px-4 py-3 hover:bg-surface-alt transition-colors text-left cursor-pointer'
                    onClick={() => toggleExpand(machine.ID)}
                  >
                    {isOpen ? (
                      <ChevronDown className='size-3.5 text-ink-faint shrink-0' />
                    ) : (
                      <ChevronRight className='size-3.5 text-ink-faint shrink-0' />
                    )}
                    <Server className='size-3.5 text-ink-faint shrink-0' />
                    <span className='font-mono text-xs text-ink flex-1'>
                      {machine.Hostname?.toLowerCase() ?? "N/A"}
                    </span>
                    <span className='text-[0.625rem] text-ink-faint'>
                      {availableCount > 0 ? `${availableCount} available` : "all added"}
                    </span>
                  </button>

                  {isOpen && (
                    <div className='border-t border-rim max-h-48 overflow-y-auto'>
                      {pools.map((pool) => {
                        const isAssigned = assignedPoolIds.has(pool.ID);
                        const isChecked = selected.has(pool.ID);
                        return (
                          <button
                            key={pool.ID}
                            type='button'
                            disabled={isAssigned}
                            className={cn(
                              "w-full flex items-center gap-3 pl-10 pr-4 py-2.5 transition-colors text-left",
                              isAssigned
                                ? "opacity-40 cursor-not-allowed"
                                : "hover:bg-surface-alt cursor-pointer"
                            )}
                            onClick={() => !isAssigned && togglePool(pool.ID)}
                          >
                            <span
                              className={cn(
                                "size-4 rounded border flex items-center justify-center shrink-0",
                                isAssigned
                                  ? "bg-surface-high border-rim"
                                  : isChecked
                                  ? "bg-accent border-accent"
                                  : "border-rim bg-surface"
                              )}
                            >
                              {(isAssigned || isChecked) && <Check className='size-2.5 text-white' />}
                            </span>
                            <span className='font-mono text-xs text-ink flex-1'>{pool.Name}</span>
                            {isAssigned && <span className='text-[0.625rem] text-ink-faint'>Added</span>}
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}

        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button
            type='button'
            loading={submitting}
            disabled={!hasChanges || submitting}
            onClick={() => void handleSubmit()}
          >
            Add Pools
          </Button>
        </div>
      </div>
    </Modal>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function ApplicationDetail() {
  const { appId } = useParams<{ appId: string }>();
  const navigate = useNavigate();
  const { activeProject } = useProjectStore();
  const queryClient = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);
  const [bulkActing, setBulkActing] = useState<"restart" | "recycle" | null>(null);

  const numericId = Number(appId);
  const queryKey = ["application", activeProject?.Key, numericId];

  const {
    data: app,
    isLoading,
    isError,
  } = useQuery({
    queryKey,
    enabled: !!activeProject && !!numericId,
    refetchInterval: 5_000,
    queryFn: async () => {
      const { data } = await applicationService.get(activeProject!.Key, numericId);
      return data.application;
    },
  });

  usePageTitle(app?.Name);

  const assignedPoolIds = new Set((app?.app_pools ?? []).map((p) => p.ID));

  async function runAllCmd(cmd: "restart" | "recycle") {
    const pools = app?.app_pools ?? [];
    if (!pools.length) return;
    setBulkActing(cmd);
    try {
      await Promise.all(
        pools.map((pool) => appPoolService.command(activeProject!.Key, pool.machine.ID, pool.ID, cmd))
      );
      queryClient.invalidateQueries({ queryKey });
      toast.success(`All pools ${cmd}ed successfully.`);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        `${cmd} failed. Please try again.`;
      toast.error(msg);
    } finally {
      setBulkActing(null);
    }
  }

  function handleSuccess() {
    queryClient.invalidateQueries({ queryKey });
    queryClient.invalidateQueries({ queryKey: ["applications", activeProject?.Key] });
  }

  if (!activeProject) {
    return (
      <div className='flex flex-col items-center justify-center py-24 text-center'>
        <p className='text-sm text-ink-faint'>No project selected.</p>
      </div>
    );
  }

  return (
    <div className='space-y-5'>
      <button
        type='button'
        onClick={() => navigate("/applications")}
        className='flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink transition-colors cursor-pointer'
      >
        <ArrowLeft className='size-3.5' />
        Applications
      </button>

      {isLoading ? (
        <div className='space-y-4'>
          <div className='h-20 rounded-lg border border-rim bg-surface animate-pulse' />
          <div className='h-48 rounded-lg border border-rim bg-surface animate-pulse' />
        </div>
      ) : isError ? (
        <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
          <AlertCircle className='size-4 shrink-0' />
          Failed to load application. Check your connection and try again.
        </div>
      ) : app ? (
        <>
          {/* Header card */}
          {(() => {
            const total = app.app_pools?.length ?? 0;
            const healthy = (app.app_pools ?? []).filter((p) => p.State === "Started").length;
            const healthColor =
              total === 0
                ? "text-ink-faint"
                : healthy === total
                ? "text-success"
                : healthy === 0
                ? "text-danger"
                : "text-warning";
            const dotColor =
              total === 0
                ? "bg-ink-faint"
                : healthy === total
                ? "bg-success"
                : healthy === 0
                ? "bg-danger"
                : "bg-warning";
            return (
              <div className='flex items-center justify-between rounded-lg border border-rim bg-surface px-5 py-4'>
                <div className='flex items-center gap-3'>
                  <div className='flex size-9 shrink-0 items-center justify-center rounded-lg border border-rim bg-surface-alt'>
                    <AppWindow className='size-4 text-ink-faint' />
                  </div>
                  <div>
                    <h1 className='text-sm font-semibold text-ink'>{app.Name}</h1>
                    <div className='mt-0.5 flex items-center gap-3'>
                      <span className='text-xs text-ink-faint'>
                        Created{" "}
                        {new Date(app.CreatedAt).toLocaleDateString("en-US", {
                          year: "numeric",
                          month: "short",
                          day: "numeric",
                        })}
                      </span>
                      {total > 0 && (
                        <div className='flex items-center gap-1.5'>
                          <span className={cn("size-1.5 rounded-full shrink-0", dotColor)} />
                          <span className={cn("text-xs", healthColor)}>
                            {healthy}/{total} Healthy
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
                <div className='flex items-center gap-2'>
                  {total > 0 && (
                    <>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={!!bulkActing}
                        loading={bulkActing === "restart"}
                        onClick={() => void runAllCmd("restart")}
                      >
                        <RotateCw className='size-3' />
                        Restart All
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={!!bulkActing}
                        loading={bulkActing === "recycle"}
                        onClick={() => void runAllCmd("recycle")}
                      >
                        <RefreshCw className='size-3' />
                        Recycle All
                      </Button>
                    </>
                  )}
                  <Button size='sm' onClick={() => setModalOpen(true)}>
                    <Plus className='size-3.5' />
                    Add App Pools
                  </Button>
                </div>
              </div>
            );
          })()}

          {/* Pool list - capped height, sticky header */}
          {!app.app_pools?.length ? (
            <EmptyPools onAdd={() => setModalOpen(true)} />
          ) : (
            <div className='max-h-72 overflow-y-auto rounded-lg border border-rim'>
              <PoolList
                pools={app.app_pools}
                projectKey={activeProject.Key}
                appId={numericId}
                appName={app.Name}
                queryKey={queryKey}
              />
            </div>
          )}

          {/* Logs - only shown when pools are assigned */}
          {(app.app_pools?.length ?? 0) > 0 && (
            <LogsViewer
              projectKey={activeProject.Key}
              appId={numericId}
              machines={[
                ...new Map(
                  (app.app_pools ?? []).map((p) => [
                    p.machine.ID,
                    { id: p.machine.ID, hostname: p.machine.Hostname?.toLowerCase() ?? "N/A" },
                  ])
                ).values(),
              ]}
              pools={(app.app_pools ?? []).map((p) => ({
                id: p.ID,
                name: p.Name,
                machineId: p.machine.ID,
              }))}
            />
          )}
        </>
      ) : null}

      {activeProject && (
        <AddAppPoolModal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          projectKey={activeProject.Key}
          appId={numericId}
          assignedPoolIds={assignedPoolIds}
          onSuccess={handleSuccess}
        />
      )}
    </div>
  );
}
