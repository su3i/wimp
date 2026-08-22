import { useRef, useState } from "react";
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
  ExternalLink,
  CheckCircle2,
  ScrollText,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { RowMenu } from "@/components/ui/RowMenu";
import type { RowMenuItem } from "@/components/ui/RowMenu";
import { Button } from "@/components/ui/Button";
import { InfoTooltip } from "@/components/ui/InfoTooltip";
import { Modal } from "@/components/ui/Modal";
import { useProjectStore } from "@/store/project";
import { LogsViewer, type LogsViewerHandle } from "@/components/application/LogsViewer";
import { applicationService } from "@/services/application.service";
import { appPoolService } from "@/services/appPool.service";
import { machineService } from "@/services/machine.service";
import { prometheusService } from "@/services/prometheus.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import { useActionCooldown } from "@/utils/useActionCooldown";
import type { AppPoolWithDetails, ApplicationDetail as AppDetailType, MachineWithPools } from "@/types";

type PoolCmd = "start" | "stop" | "restart" | "recycle";

const CONFIRM_POOL_CMDS: PoolCmd[] = ["stop", "restart", "recycle"];

const poolConfirmCopy: Record<Exclude<PoolCmd, "start">, { title: string; verb: string }> = {
  stop: { title: "Stop App Pool", verb: "stop" },
  restart: { title: "Restart App Pool", verb: "restart" },
  recycle: { title: "Recycle App Pool", verb: "recycle" },
};

const TH = "px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";

// Where the tree's vertical connector sits, in pixels from the left edge of the list. Set
// to the centre of the host row's icon (pl-4 = 16px, plus half of a 14px icon), so the
// line the children hang off genuinely descends from their parent. Shared by the stub
// under the host row and the spine beside each pool - they have to agree or the tree
// visibly kinks at every group boundary.
const TREE_SPINE_X = 23;

const poolStatusCfg: Record<string, { dot: string; text: string }> = {
  Started: { dot: "bg-success", text: "text-success" },
  Stopped: { dot: "bg-danger", text: "text-danger" },
  Starting: { dot: "bg-warning", text: "text-warning" },
  Stopping: { dot: "bg-warning", text: "text-warning" },
};

function PoolStatus({ state, offline }: { state: string; offline?: boolean }) {
  if (offline) {
    return (
      <div className='flex items-center gap-2'>
        <span className='size-1.5 rounded-full shrink-0 bg-ink-faint' />
        <span className='text-xs text-ink-faint'>Unknown</span>
      </div>
    );
  }
  const cfg = poolStatusCfg[state] ?? { dot: "bg-ink-faint", text: "text-ink-faint" };
  return (
    <div className='flex items-center gap-2'>
      <span className={cn("size-1.5 rounded-full shrink-0", cfg.dot)} />
      <span className={cn("text-xs", cfg.text)}>{state}</span>
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
  app: AppDetailType;
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
    <Modal open={open} onClose={onClose} title='Edit Application'>
      <div className='space-y-4'>
        <div className='space-y-1.5'>
          <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>Name</label>
          <input
            type='text'
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            className='w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink placeholder:text-ink-faint focus:outline-none focus:border-accent'
          />
        </div>

        <div className='space-y-1.5'>
          <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
            Health Check URL
          </label>
          <input
            type='text'
            value={hcUrl}
            onChange={(e) => setHcUrl(e.target.value)}
            placeholder='https://example.com/health'
            className='w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink placeholder:text-ink-faint focus:outline-none focus:border-accent'
          />
        </div>

        {hcUrl.trim() && (
          <div className='space-y-1.5'>
            <label className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>Health Check Interval</label>
            <select
              value={hcInterval}
              onChange={(e) => setHcInterval(Number(e.target.value))}
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
          <Button type='button' variant='outline' onClick={onClose} disabled={saving}>Cancel</Button>
          <Button type='button' loading={saving} disabled={saving || !name.trim()} onClick={() => void handleSave()}>
            Save
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function fmtInterval(s: number) {
  if (s < 60) return `${s}s`;
  return `${Math.round(s / 60)}m`;
}

function sslDaysLeft(unixTs: number) {
  return Math.floor((unixTs - Date.now() / 1000) / 86400);
}

// Certificate expiry as a value and a tone, rather than a self-rendering badge - the cards
// below all present one figure the same way, and SSL should not be the odd one out.
function sslDisplay(days: number | null): { value: string; tone: string } {
  if (days == null) return { value: "N/A", tone: "text-ink" };
  if (days < 0) return { value: "Expired", tone: "text-danger" };
  if (days <= 14) return { value: `${days}d`, tone: "text-warning" };
  return { value: `${days}d`, tone: "text-success" };
}

// Shared shell for the health figures. Same proportions as the Overview page's stat tiles
// so the two pages read as one product: label pinned top, figure bottom, fixed floor height
// so a row of them stays even regardless of how long each value is.
const HEALTH_CARD =
  "rounded-lg border border-rim bg-surface px-[18px] py-[18px] flex flex-col justify-between gap-3 min-h-[112px]";

function HealthCard({
  label,
  value,
  sub,
  info,
  icon: Icon,
  tone = "text-ink",
}: {
  label: string;
  value: string;
  sub: string;
  info: string;
  // Rendered beside the figure. Only the healthy state carries one - a green tick reads
  // instantly, whereas an icon on every state would just be noise next to an already
  // colour-coded value.
  icon?: LucideIcon;
  tone?: string;
}) {
  return (
    <div className={HEALTH_CARD}>
      <span className='flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint leading-none'>
        {label}
        <InfoTooltip text={info} />
      </span>
      <span className={cn("flex items-center gap-2 font-mono text-[24px] font-semibold leading-none", tone)}>
        {Icon && <Icon className='size-5 shrink-0' />}
        {value}
      </span>
      <span className='text-[0.625rem] text-ink-faint'>{sub}</span>
    </div>
  );
}

function HealthMonitor({ app }: { app: AppDetailType }) {
  const url = app.HealthCheckURL;
  const interval = app.HealthCheckIntervalSeconds ?? 60;
  const appId = String(app.ID);
  const isHttps = url?.toLowerCase().startsWith("https://");
  const promEnabled = prometheusService.isConfigured();

  const idFilter = `job="blackbox_http", application_id="${appId}"`;

  const { data: statusData } = useQuery({
    queryKey: ["hc-status", appId],
    enabled: promEnabled,
    refetchInterval: 30_000,
    queryFn: () => prometheusService.instant(`probe_success{${idFilter}}`),
  });

  const { data: uptimeData } = useQuery({
    queryKey: ["hc-uptime", appId],
    enabled: promEnabled,
    refetchInterval: 60_000,
    queryFn: () => prometheusService.instant(`avg_over_time(probe_success{${idFilter}}[30d])`),
  });

  const { data: sslData } = useQuery({
    queryKey: ["hc-ssl", appId],
    enabled: promEnabled && !!isHttps,
    refetchInterval: 60_000,
    queryFn: () => prometheusService.instant(`probe_ssl_earliest_cert_expiry{${idFilter}}`),
  });

  const statusVal = statusData?.[0]?.value[1];
  const status: "up" | "down" | "unknown" =
    statusVal === "1" ? "up" : statusVal === "0" ? "down" : "unknown";

  const uptimeRatio = uptimeData?.[0]?.value[1];
  const uptimePct = uptimeRatio != null ? `${(Number(uptimeRatio) * 100).toFixed(2)}%` : "N/A";

  const sslTs = sslData?.[0]?.value[1];
  const sslDays = sslTs != null ? sslDaysLeft(Number(sslTs)) : null;
  const ssl = sslDisplay(sslDays);

  const statusColor =
    status === "up" ? "text-success" : status === "down" ? "text-danger" : "text-ink-faint";

  return (
    // Four separate cards rather than one divided panel, matching the Overview page. The
    // endpoint URL lives in the page header, so there is no panel chrome left to justify
    // boxing these together.
    <div className='grid grid-cols-2 sm:grid-cols-4 gap-[14px]'>
      <HealthCard
        label='Status'
        value={promEnabled ? (status === "up" ? "Up" : status === "down" ? "Down" : "No data") : "N/A"}
        sub={promEnabled ? "last probe" : "no prometheus"}
        info="Whether the most recent probe of this application's health check URL succeeded."
        icon={promEnabled && status === "up" ? CheckCircle2 : undefined}
        tone={promEnabled ? statusColor : "text-ink"}
      />
      <HealthCard
        label='Uptime'
        value={promEnabled ? uptimePct : "N/A"}
        sub='last 30 days'
        info='Share of health check probes that succeeded over the last 30 days.'
      />
      <HealthCard
        label='SSL Expiry'
        value={promEnabled && isHttps ? ssl.value : "N/A"}
        sub={isHttps ? "until expiry" : "http endpoint"}
        info="Days left on the TLS certificate serving this endpoint. Only measured for https URLs."
        tone={promEnabled && isHttps ? ssl.tone : "text-ink"}
      />
      <HealthCard
        label='Interval'
        value={fmtInterval(interval)}
        sub='probe frequency'
        info='How often the health check runs against this endpoint.'
      />
    </div>
  );
}

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
        Add App Pool
      </Button>
    </div>
  );
}

function EditLogPathModal({
  open,
  onClose,
  pool,
  projectKey,
  appId,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
  pool: AppPoolWithDetails;
  projectKey: string;
  appId: number;
  onSuccess: () => void;
}) {
  const [logPath, setLogPath] = useState(pool.log_path ?? "");
  const [saving, setSaving] = useState(false);

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
            className='w-full h-8 px-3 rounded-md border border-rim bg-surface text-xs text-ink placeholder:text-ink-faint focus:outline-none focus:border-accent'
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

interface PoolGroup {
  machineId: number;
  hostname: string;
  online: boolean;
  pools: AppPoolWithDetails[];
}

// Collapses the flat pool list the API returns into one node per host, sorted by hostname
// and with each host's pools sorted by name, so the ordering is stable across the 5s
// refetch instead of following whatever order the query happened to return.
function groupPoolsByMachine(pools: AppPoolWithDetails[]): PoolGroup[] {
  const byMachine = new Map<number, PoolGroup>();
  for (const pool of pools) {
    const machineId = pool.machine?.ID ?? 0;
    let group = byMachine.get(machineId);
    if (!group) {
      group = {
        machineId,
        hostname: pool.machine?.Hostname?.toLowerCase() || "Unknown host",
        online: pool.machine?.Status === "online",
        pools: [],
      };
      byMachine.set(machineId, group);
    }
    group.pools.push(pool);
  }
  const groups = [...byMachine.values()];
  for (const group of groups) group.pools.sort((a, b) => a.Name.localeCompare(b.Name));
  return groups.sort((a, b) => a.hostname.localeCompare(b.hostname));
}

function PoolList({
  pools,
  projectKey,
  appId,
  queryKey,
  onViewLogs,
}: {
  pools: AppPoolWithDetails[];
  projectKey: string;
  appId: number;
  queryKey: unknown[];
  onViewLogs: (pool: AppPoolWithDetails) => void;
}) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [acting, setActing] = useState<Record<number, string>>({});

  const [editingPool, setEditingPool] = useState<AppPoolWithDetails | null>(null);
  const [removeTarget, setRemoveTarget] = useState<AppPoolWithDetails | null>(null);
  const [removing, setRemoving] = useState(false);
  const [confirmTarget, setConfirmTarget] = useState<{ pool: AppPoolWithDetails; cmd: PoolCmd } | null>(null);
  const cooldown = useActionCooldown();

  async function runCmd(pool: AppPoolWithDetails, cmd: PoolCmd) {
    setActing((prev) => ({ ...prev, [pool.ID]: cmd }));
    cooldown.start(pool.ID);
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

  function requestCmd(pool: AppPoolWithDetails, cmd: PoolCmd) {
    if (CONFIRM_POOL_CMDS.includes(cmd)) {
      setConfirmTarget({ pool, cmd });
    } else {
      void runCmd(pool, cmd);
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

  // Pools are grouped under the host they run on rather than repeating the hostname in a
  // column on every row - one application commonly has the same pool name on several
  // machines, so the machine is the real parent in this hierarchy, not a per-row attribute.
  const groups = groupPoolsByMachine(pools);
  const cols = "grid-cols-[1.8fr_0.9fr_1.4fr_auto]";

  // No items-center on the grid - see Applications.tsx for why (each cell stretches to
  // the full row height via grid's default align-items:stretch, then centers its own
  // content with its own flex items-center; centering at the grid level instead shrinks
  // each cell to its own content height first, which looks inconsistent).
  return (
    <div className={`grid ${cols} overflow-hidden`}>
      <div className={cn(TH, "sticky top-0 z-10 border-b border-rim bg-surface-alt")}>Host / App Pool</div>
      <div className={cn(TH, "sticky top-0 z-10 border-b border-rim bg-surface-alt")}>Status</div>
      <div className={cn(TH, "sticky top-0 z-10 border-b border-rim bg-surface-alt")}>Log Path</div>
      <div className='sticky top-0 z-10 border-b border-rim bg-surface-alt px-4 py-2.5' />

      {groups.map((group, groupIdx) => {
        // The container draws its own bottom border, so the last row inside it must not
        // draw one too or the two stack into a double line.
        const isLastGroup = groupIdx === groups.length - 1;
        return (
          <div key={group.machineId} className='contents'>
            {/* Host row - the parent node, always showing its children, and a link to that
                host's own page. The stub at the bottom is what makes this read as a tree
                rather than as two unrelated row styles: without it the children's spine
                starts in mid-air below the border, connected to nothing. It descends from
                directly under the host icon, which is where TREE_SPINE_X is measured to. */}
            <button
              type='button'
              // groupPoolsByMachine buckets pools whose machine record is missing under id
              // 0, and there is no host page to send those to.
              disabled={!group.machineId}
              onClick={() => navigate(`/hosts/${group.machineId}`)}
              title={group.machineId ? `Open ${group.hostname}` : undefined}
              className={cn(
                'group/host relative col-span-full flex items-center gap-2.5 border-b border-rim bg-surface-alt/60 py-2.5 pl-4 pr-4 text-left transition-colors',
                group.machineId ? 'cursor-pointer hover:bg-surface-alt' : 'cursor-default',
              )}
            >
              <Server className='size-3.5 shrink-0 text-ink-faint' />
              <span className='font-mono text-xs text-ink truncate underline decoration-transparent underline-offset-2 transition-colors group-hover/host:decoration-current'>
                {group.hostname}
              </span>
              {!!group.machineId && (
                <ChevronRight className='size-3.5 shrink-0 text-ink-faint opacity-0 transition-opacity group-hover/host:opacity-100' />
              )}
              <span
                className='absolute bottom-0 h-2.5 w-px bg-rim'
                style={{ left: TREE_SPINE_X }}
              />
            </button>

            {group.pools.map((pool, idx) => {
              const busy = acting[pool.ID];
              const transitional = pool.State === "Starting" || pool.State === "Stopping";
              const cooling = cooldown.isCooling(pool.ID) || transitional;
              const offline = pool.machine?.Status !== "online";
              const started = pool.State === "Started";
              const isLastChild = idx === group.pools.length - 1;
              const cellBorder = !(isLastGroup && isLastChild) && "border-b border-rim";
              const hasLogPath = !!pool.log_path;
              const menuItems: RowMenuItem[] = [
                // Only offered once a log path is set - without one there is nothing to open.
                ...(hasLogPath
                  ? [{ icon: ScrollText, label: "View Logs", onClick: () => onViewLogs(pool) }]
                  : []),
                { icon: Pencil, label: "Edit", onClick: () => setEditingPool(pool) },
                { type: "separator" },
                ...(offline
                  ? []
                  : started
                    ? [
                        {
                          icon: Square,
                          label: "Stop",
                          variant: "danger" as const,
                          onClick: () => requestCmd(pool, "stop"),
                        },
                        { icon: RotateCw, label: "Restart", onClick: () => requestCmd(pool, "restart") },
                        { icon: RefreshCw, label: "Recycle", onClick: () => requestCmd(pool, "recycle") },
                      ]
                    : [{ icon: Play, label: "Start", onClick: () => requestCmd(pool, "start") }]),
                ...(offline ? [] : [{ type: "separator" as const }]),
                { icon: Trash2, label: "Remove", variant: "danger", onClick: () => setRemoveTarget(pool) },
              ];
              return (
                <div key={pool.ID} className={cn("contents group", cooling && "opacity-50")}>
                  <div className={cn("flex min-w-0 items-stretch pr-4 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                    {/* Tree guide: a vertical spine dropping from the host row above,
                        stopping halfway down on the last child so the branch visibly
                        ends, plus a horizontal elbow into the pool itself. */}
                    <span className='relative w-5 shrink-0' style={{ marginLeft: TREE_SPINE_X }}>
                      <span
                        className={cn("absolute left-0 top-0 w-px bg-rim", isLastChild ? "h-1/2" : "h-full")}
                      />
                      <span className='absolute left-0 top-1/2 h-px w-4 bg-rim' />
                    </span>
                    <span className='flex min-w-0 items-center gap-2.5 py-3'>
                      <span className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                        <Cpu className='size-3 text-ink-faint' />
                      </span>
                      <span className='font-mono text-xs text-ink truncate'>{pool.Name}</span>
                    </span>
                  </div>

                  <div className={cn("flex items-center px-4 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                    <PoolStatus state={pool.State} offline={offline} />
                  </div>

                  <div className={cn("flex min-w-0 items-center px-4 py-3.5 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)} title={pool.log_path ?? undefined}>
                    {hasLogPath ? (
                      <button
                        type='button'
                        onClick={() => onViewLogs(pool)}
                        className='block min-w-0 cursor-pointer truncate text-left text-xs text-ink-dim underline decoration-transparent underline-offset-2 transition-colors hover:text-ink hover:decoration-current'
                      >
                        {pool.log_path}
                      </button>
                    ) : (
                      <button
                        type='button'
                        onClick={() => setEditingPool(pool)}
                        className='cursor-pointer text-xs text-ink-faint underline decoration-dotted underline-offset-2 transition-colors hover:text-ink-dim'
                      >
                        Set a log path
                      </button>
                    )}
                  </div>

                  <div className={cn("flex items-center justify-center px-2 py-3 group-hover:bg-surface-alt transition-colors duration-100", cellBorder)}>
                    <RowMenu items={menuItems} disabled={!!busy || cooling} />
                  </div>
                </div>
              );
            })}
          </div>
        );
      })}

      {editingPool && (
        <EditLogPathModal
          open={!!editingPool}
          onClose={() => setEditingPool(null)}
          pool={editingPool}
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

      <ConfirmModal
        open={!!confirmTarget}
        title={confirmTarget ? poolConfirmCopy[confirmTarget.cmd as Exclude<PoolCmd, "start">].title : ""}
        description={
          confirmTarget
            ? `Are you sure you want to ${poolConfirmCopy[confirmTarget.cmd as Exclude<PoolCmd, "start">].verb} "${confirmTarget.pool.Name}"?`
            : ""
        }
        confirmLabel={
          confirmTarget ? poolConfirmCopy[confirmTarget.cmd as Exclude<PoolCmd, "start">].title.split(" ")[0] : "Confirm"
        }
        onClose={() => setConfirmTarget(null)}
        onConfirm={() => {
          if (confirmTarget) void runCmd(confirmTarget.pool, confirmTarget.cmd);
          setConfirmTarget(null);
        }}
      />
    </div>
  );
}

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
      const { data } = await machineService.list(projectKey, { per_page: 100 });
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
        "Failed to add app pool(s). Please try again.";
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  }

  const hasChanges = selected.size > 0;

  return (
    <Modal open={open} onClose={onClose} title='Add App Pool'>
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
                      {availableCount > 0 ? `${availableCount} available` : ""}
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

export function ApplicationDetail() {
  const { appId } = useParams<{ appId: string }>();
  const navigate = useNavigate();
  const { activeProject } = useProjectStore();
  const queryClient = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);
  const logsRef = useRef<LogsViewerHandle>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [bulkActing, setBulkActing] = useState<"restart" | "recycle" | null>(null);
  const [bulkConfirm, setBulkConfirm] = useState<"restart" | "recycle" | null>(null);

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

  const ROLLING_RESTART_DELAY_MS = 10_000;

  // Rolling restart/recycle: fires one pool at a time with a fixed delay between each,
  // rather than hitting every pool at once (which would take the whole app down
  // simultaneously). Gives immediate feedback that the rollout started instead of
  // blocking until every pool finishes.
  async function runAllCmd(cmd: "restart" | "recycle") {
    const pools = app?.app_pools ?? [];
    if (!pools.length) return;
    setBulkActing(cmd);
    toast.success(`Rolling ${cmd} started for ${pools.length} pool${pools.length > 1 ? "s" : ""}.`);

    let failures = 0;
    for (let i = 0; i < pools.length; i++) {
      const pool = pools[i];
      try {
        await appPoolService.command(activeProject!.Key, pool.machine.ID, pool.ID, cmd);
      } catch {
        failures++;
      }
      queryClient.invalidateQueries({ queryKey });
      if (i < pools.length - 1) {
        await new Promise((resolve) => setTimeout(resolve, ROLLING_RESTART_DELAY_MS));
      }
    }

    setBulkActing(null);
    if (failures > 0) {
      toast.error(`${failures} of ${pools.length} pools failed to ${cmd}.`);
    } else {
      toast.success(`Rolling ${cmd} complete for all ${pools.length} pools.`);
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
          <div className='flex items-start gap-4 animate-pulse'>
            <div className='mt-1 size-9 rounded-lg border border-rim bg-surface-high shrink-0' />
            <div>
              <div className='flex items-center gap-2.5 mb-3'>
                <div className='h-4 w-40 rounded bg-surface-high' />
                <div className='h-5 w-16 rounded-full bg-surface-high' />
              </div>
              <div className='h-2.5 w-28 rounded bg-surface-high' />
            </div>
          </div>
          <div className='h-48 rounded-lg border border-rim bg-surface animate-pulse' />
        </div>
      ) : isError ? (
        <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
          <AlertCircle className='size-4 shrink-0' />
          Failed to load application. Check your connection and try again.
        </div>
      ) : app ? (
        <>
          {/* Header */}
          {(() => {
            const total = app.app_pools?.length ?? 0;
            const healthy = (app.app_pools ?? []).filter(
              (p) => p.State === "Started" && p.machine?.Status === "online"
            ).length;
            const dotColor =
              total === 0 ? "bg-ink-faint"
              : healthy === total ? "bg-success"
              : healthy === 0 ? "bg-danger"
              : "bg-warning";
            return (
              <div className='flex items-start justify-between gap-6'>
                <div className='flex items-start gap-4'>
                  <div className='mt-1 flex size-9 shrink-0 items-center justify-center rounded-lg border border-rim bg-surface-alt'>
                    <AppWindow className='size-4 text-ink-faint' />
                  </div>
                  <div>
                    <div className='flex items-center gap-2.5 mb-3'>
                      <h1 className='text-base font-semibold text-ink'>{app.Name}</h1>
                      {total > 0 && (
                        <span className={cn(
                          'inline-flex items-center gap-1.5 text-xs font-medium',
                          healthy === total ? 'text-success' :
                          healthy === 0    ? 'text-danger'   :
                                             'text-warning'
                        )}>
                          <span className={cn('size-1.5 rounded-full shrink-0', dotColor)} />
                          {healthy}/{total} healthy
                        </span>
                      )}
                    </div>
                    {app.HealthCheckURL ? (
                      <a
                        href={app.HealthCheckURL}
                        target='_blank'
                        rel='noopener noreferrer'
                        className='flex min-w-0 items-center gap-1.5 text-xs text-ink-dim hover:text-ink underline underline-offset-2 transition-colors'
                      >
                        <span className='truncate max-w-[420px]'>{app.HealthCheckURL}</span>
                        <ExternalLink className='size-3 shrink-0' />
                      </a>
                    ) : (
                      <p className='text-xs text-ink-faint'>
                        Created{' '}
                        {new Date(app.CreatedAt).toLocaleDateString('en-US', {
                          year: 'numeric',
                          month: 'short',
                          day: 'numeric',
                        })}
                      </p>
                    )}
                  </div>
                </div>
                <div className='flex items-center gap-2 shrink-0 pt-1'>
                  {total > 0 && (
                    <>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={!!bulkActing}
                        loading={bulkActing === "restart"}
                        onClick={() => setBulkConfirm("restart")}
                      >
                        <RotateCw className='size-3' />
                        Restart All
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={!!bulkActing}
                        loading={bulkActing === "recycle"}
                        onClick={() => setBulkConfirm("recycle")}
                      >
                        <RefreshCw className='size-3' />
                        Recycle All
                      </Button>
                    </>
                  )}
                  <Button size='sm' variant='outline' onClick={() => setEditOpen(true)}>
                    <Pencil className='size-3' />
                    Edit
                  </Button>
                  <Button
                    size='sm'
                    onClick={() => setModalOpen(true)}
                  >
                    <Plus className='size-3.5' />
                    Add App Pool
                  </Button>
                </div>
              </div>
            );
          })()}

          {/* Health monitor - shown when health check URL is configured */}
          {app.HealthCheckURL && <HealthMonitor app={app} />}

          {/* Pool list - sticky header, flows to its full height */}
          {!app.app_pools?.length ? (
            <EmptyPools onAdd={() => setModalOpen(true)} />
          ) : (
            <div className='rounded-lg border border-rim'>
              <PoolList
                pools={app.app_pools}
                projectKey={activeProject.Key}
                appId={numericId}
                queryKey={queryKey}
                onViewLogs={(pool) => logsRef.current?.openFor(pool.machine.ID, pool.ID)}
              />
            </div>
          )}

          {/* Logs - only shown when pools are assigned */}
          {(app.app_pools?.length ?? 0) > 0 && (
            <LogsViewer
              ref={logsRef}
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

      {activeProject && app && (
        <EditApplicationModal
          open={editOpen}
          onClose={() => setEditOpen(false)}
          app={app}
          projectKey={activeProject.Key}
          onSuccess={handleSuccess}
        />
      )}

      <ConfirmModal
        open={!!bulkConfirm}
        title={bulkConfirm === "recycle" ? "Recycle App Pools" : "Restart App Pools"}
        description={`Are you sure you want to ${bulkConfirm} all app pools in this application? They'll ${bulkConfirm === "recycle" ? "recycle" : "restart"} one after another rather than all at once.`}
        confirmLabel={bulkConfirm === "recycle" ? "Recycle All" : "Restart All"}
        onClose={() => setBulkConfirm(null)}
        onConfirm={() => {
          if (bulkConfirm) void runAllCmd(bulkConfirm);
          setBulkConfirm(null);
        }}
      />
    </div>
  );
}
