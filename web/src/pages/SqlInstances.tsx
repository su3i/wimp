import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { AlertCircle, Database, Layers, Play, RotateCw, Shuffle, Square } from "lucide-react";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { RowMenu } from "@/components/ui/RowMenu";
import { Pagination } from "@/components/ui/Pagination";
import { usePaged } from "@/utils/usePaged";
import { InfoTooltip } from "@/components/ui/InfoTooltip";
import { SqlStatusBadge } from "@/components/sql/SqlBadges";
import { MoveInstanceModal } from "@/components/sql/MoveInstanceModal";
import { useProjectStore } from "@/store/project";
import { sqlInstanceService } from "@/services/sqlInstance.service";
import { prometheusService } from "@/services/prometheus.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import { formatUptime } from "@/utils/time";
import { apiError, groupInstances, sqlRelease, type SqlInstanceGroup } from "@/utils/sql";

const TH = "px-5 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint";
// Header and rows share one template so the two grids cannot size their tracks differently.
const COLS = "grid-cols-[1.6fr_1.4fr_1.6fr_0.9fr_1.1fr_auto]";

function nodeOf(g: SqlInstanceGroup) {
  const i = g.current;
  return ((i.Clustered ? i.OwnerNode || i.machine.Hostname : i.machine.Hostname) ?? "").toLowerCase();
}

function TableSkeleton() {
  return (
    <div className='rounded-lg border border-rim overflow-hidden'>
      <div className='border-b border-rim bg-surface-alt px-5 py-2.5 flex gap-8'>
        {["w-28", "w-36", "w-32", "w-20", "w-24"].map((w, i) => (
          <div key={i} className={cn("h-2.5 rounded bg-surface-high animate-pulse", w)} />
        ))}
      </div>
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className='flex items-center gap-6 px-5 py-4 border-b border-rim last:border-0 animate-pulse'>
          <div className='size-6 rounded border border-rim bg-surface-high' />
          <div className='flex-[2] h-3 rounded bg-surface-high' />
          <div className='flex-[2] h-2.5 rounded bg-surface-high' />
          <div className='flex-1 h-2.5 rounded bg-surface-high' />
          <div className='flex-1 h-2.5 rounded bg-surface-high' />
        </div>
      ))}
    </div>
  );
}

function CenteredState({ icon: Icon, title, body }: { icon: typeof Database; title: string; body: string }) {
  return (
    <div className='flex flex-col items-center justify-center py-20 rounded-lg border border-rim bg-surface text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface-alt'>
        <Icon className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>{title}</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>{body}</p>
    </div>
  );
}

export function SqlInstances() {
  usePageTitle("SQL Server Instances");
  const { activeProject } = useProjectStore();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [restartTarget, setRestartTarget] = useState<SqlInstanceGroup | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [moveTarget, setMoveTarget] = useState<SqlInstanceGroup | null>(null);

  const { data: rows, isLoading, isError } = useQuery({
    queryKey: ["project-sql-instances", activeProject?.Key],
    enabled: !!activeProject,
    refetchInterval: 5_000,
    queryFn: async () => (await sqlInstanceService.listProject(activeProject!.Key)).data.sql_instances ?? [],
  });
  const groups = useMemo(() => groupInstances(rows ?? []), [rows]);
  const paged = usePaged(groups);

  // Uptime needs SQL read access; without it the column reads N/A rather than guessing.
  const hostIds = [...new Set((rows ?? []).map((r) => r.MachineID))].join("|");
  const { data: uptimeResult } = useQuery({
    queryKey: ["sql-uptime", hostIds],
    enabled: prometheusService.isConfigured() && !!hostIds,
    refetchInterval: 60_000,
    queryFn: () => prometheusService.instant(`wimp_mssql_uptime_seconds{machine_id=~"${hostIds}"}`),
  });
  const uptime = (g: SqlInstanceGroup) => {
    const hosts = new Set(g.rows.map((r) => String(r.MachineID)));
    const hit = (uptimeResult ?? []).find((r) => hosts.has(r.metric.machine_id) && r.metric.sql_instance === g.current.Name);
    return hit ? Number(hit.value[1]) : null;
  };

  async function run(g: SqlInstanceGroup, cmd: "start" | "restart") {
    setBusy(g.key);
    const pending = toast.loading(`${cmd === "start" ? "Starting" : "Restarting"} ${g.name}...`);
    try {
      await sqlInstanceService.command(activeProject!.Key, g.current.MachineID, g.current.Name, cmd);
      toast.success(`${g.name} ${cmd === "start" ? "started" : "restarted"}.`, { id: pending });
      queryClient.invalidateQueries({ queryKey: ["project-sql-instances", activeProject?.Key] });
    } catch (err) {
      toast.error(apiError(err, "Command failed."), { id: pending });
    } finally {
      setBusy(null);
    }
  }

  if (!activeProject) {
    return <CenteredState icon={Layers} title='No project selected' body='Select a project from the sidebar to view its SQL Server instances.' />;
  }

  return (
    <div className='space-y-5'>
      <div>
        <h1 className='text-base font-semibold text-ink'>SQL Server Instances</h1>
        <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
        <p className='mt-1 max-w-xl text-[0.6875rem] text-ink-faint'>
          Every SQL Server instance discovered on this project's hosts.
        </p>
      </div>

      {isLoading ? (
        <TableSkeleton />
      ) : isError ? (
        <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
          <AlertCircle className='size-4 shrink-0' />
          Failed to load SQL Server instances. Check your connection and try again.
        </div>
      ) : !groups.length ? (
        <CenteredState icon={Database} title='No SQL Server instances' body='No host in this project has reported a SQL Server instance yet.' />
      ) : (
        // No overflow-hidden here: it would clip the info tooltips on the last row. The header
        // and last row carry the rounding instead.
        <div className='rounded-lg border border-rim'>
          <div className={cn(COLS, "grid rounded-t-lg border-b border-rim bg-surface-alt")}>
            <div className={TH}>Name</div>
            <div className={TH}>Version</div>
            <div className={TH}>Node</div>
            <div className={TH}>Uptime</div>
            <div className={TH}>Status</div>
            <div className='px-4 py-2.5' />
          </div>

          {paged.items.map((g) => {
            const i = g.current;
            const offline = i.machine.Status !== "online";
            const running = i.State === "Running";
            const up = uptime(g);
            return (
              <div
                key={g.key}
                onClick={() => navigate(`/sql-instances/${i.ID}`)}
                className={cn(
                  COLS,
                  "grid items-center border-b border-rim last:border-0 last:rounded-b-lg hover:bg-surface-alt transition-colors duration-100 cursor-pointer",
                  busy === g.key && "opacity-50",
                )}
              >
                <div className='flex min-w-0 items-center gap-3 px-5 py-3.5'>
                  <div className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high'>
                    <Database className='size-3 text-ink-faint' />
                  </div>
                  <span className='font-mono text-xs text-ink truncate'>{g.name.toLowerCase()}</span>
                </div>

                <div className='min-w-0 px-5 py-3.5'>
                  <span className='block truncate text-xs text-ink-dim' title={`${i.Edition} ${i.Version}`}>
                    {sqlRelease(i.Version, i.Edition)}
                  </span>
                </div>

                <div className='flex min-w-0 items-center gap-1.5 px-5 py-3.5 whitespace-nowrap'>
                  <span className='truncate font-mono text-xs text-ink-dim'>{nodeOf(g)}</span>
                  {i.Clustered && <InfoTooltip text="Failover cluster instance" />}
                </div>

                <div className='px-5 py-3.5'>
                  <span className='text-xs text-ink-dim tabular-nums'>{up != null && running ? formatUptime(up) : "N/A"}</span>
                </div>

                <div className='px-5 py-3.5'>
                  <SqlStatusBadge instance={i} />
                </div>

                <div className='flex items-center justify-center px-2 py-3' onClick={(e) => e.stopPropagation()}>
                  <RowMenu
                    disabled={offline || !!busy}
                    items={[
                      running
                        ? { icon: RotateCw, label: "Restart", onClick: () => setRestartTarget(g) }
                        : { icon: Play, label: "Start", onClick: () => void run(g, "start") },
                      ...(i.Clustered
                        ? [{ icon: Shuffle, label: "Migrate", onClick: () => setMoveTarget(g) }]
                        : []),
                      { icon: Square, label: "Stop", variant: "danger" as const, disabled: true, onClick: () => {} },
                    ]}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}

      <Pagination page={paged.page} totalPages={paged.totalPages} total={paged.total} pageSize={paged.pageSize} onPageChange={paged.setPage} />

      <MoveInstanceModal projectKey={activeProject.Key} group={moveTarget} onClose={() => setMoveTarget(null)} />

      <ConfirmModal
        open={!!restartTarget}
        title='Restart SQL Server'
        description={
          restartTarget
            ? `Restart ${restartTarget.name.toLowerCase()}? Every database on it is unavailable until it comes back. This will not alert unless it fails to come back within 5 minutes.`
            : ""
        }
        confirmLabel='Restart'
        onClose={() => setRestartTarget(null)}
        onConfirm={() => {
          if (restartTarget) void run(restartTarget, "restart");
          setRestartTarget(null);
        }}
      />
    </div>
  );
}
