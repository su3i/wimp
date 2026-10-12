import { lazy, Suspense, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, Database, Network, Play, RotateCw, Server, Shuffle, Square, Tag } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { SqlCredentialsHint, SqlStatusBadge } from "@/components/sql/SqlBadges";
import { SqlDatabasesTab } from "@/components/sql/SqlDatabasesTab";
import { SqlActivityTab, SqlJobsTab } from "@/components/sql/SqlLiveTabs";
import { MoveInstanceModal } from "@/components/sql/MoveInstanceModal";
import { useProjectStore } from "@/store/project";
import { sqlInstanceService } from "@/services/sqlInstance.service";
import { cn } from "@/utils/cn";
import { usePageTitle } from "@/utils/usePageTitle";
import { apiError, findGroup, groupInstances, sqlRelease } from "@/utils/sql";

// Charts pull in recharts; split out so the header and tabs paint first, as on the host page.
const SqlOverviewTab = lazy(() => import("@/components/sql/SqlOverviewTab").then((m) => ({ default: m.SqlOverviewTab })));

type Tab = "overview" | "databases" | "jobs" | "queries";

function OverviewSkeleton() {
  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-4 gap-3'>
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className='h-24 rounded-lg border border-rim bg-surface animate-pulse' />
        ))}
      </div>
      <div className='grid grid-cols-2 gap-4'>
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className='h-48 rounded-lg border border-rim bg-surface animate-pulse' />
        ))}
      </div>
    </div>
  );
}

export function SqlInstanceDetail() {
  const { instanceId } = useParams<{ instanceId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { activeProject } = useProjectStore();
  const [tab, setTab] = useState<Tab>("overview");
  const [confirmRestart, setConfirmRestart] = useState(false);
  const [busy, setBusy] = useState(false);
  const [moving, setMoving] = useState(false);

  const { data: rows, isLoading } = useQuery({
    queryKey: ["project-sql-instances", activeProject?.Key],
    enabled: !!activeProject,
    refetchInterval: 5_000,
    queryFn: async () => (await sqlInstanceService.listProject(activeProject!.Key)).data.sql_instances ?? [],
  });
  const group = useMemo(() => findGroup(groupInstances(rows ?? []), Number(instanceId)), [rows, instanceId]);
  usePageTitle(group?.name.toLowerCase());

  const inst = group?.current;
  const offline = inst?.machine.Status !== "online";
  const running = inst?.State === "Running";
  const inCluster = !!inst?.machine.Cluster;

  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "databases", label: "Databases" },
    { id: "jobs", label: "Agent Jobs" },
    { id: "queries", label: "Live Queries" },
  ];
  const current = tab;

  async function run(cmd: "start" | "restart") {
    if (!inst || !activeProject) return;
    setBusy(true);
    const pending = toast.loading(`${cmd === "start" ? "Starting" : "Restarting"} ${group!.name.toLowerCase()}...`);
    try {
      await sqlInstanceService.command(activeProject.Key, inst.MachineID, inst.Name, cmd);
      toast.success(`${group!.name.toLowerCase()} ${cmd === "start" ? "started" : "restarted"}.`, { id: pending });
      queryClient.invalidateQueries({ queryKey: ["project-sql-instances", activeProject.Key] });
    } catch (err) {
      toast.error(apiError(err, "Command failed."), { id: pending });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className='space-y-5'>
      <button
        onClick={() => navigate("/sql-instances")}
        className='cursor-pointer flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink transition-colors'
      >
        <ArrowLeft className='size-3.5' />
        SQL Server Instances
      </button>

      {inst && group ? (
        <div className='flex items-start justify-between gap-6'>
          <div className='flex items-start gap-4'>
            <div className='mt-1 flex size-9 shrink-0 items-center justify-center rounded-lg border border-rim bg-surface-alt'>
              <Database className='size-4 text-ink-faint' />
            </div>
            <div>
              <div className='flex items-center gap-2.5 mb-3'>
                <h1 className='font-mono text-base font-semibold text-ink'>{group.name.toLowerCase()}</h1>
                <SqlStatusBadge instance={inst} />
              </div>
              <div className='space-y-2'>
                <p className='flex items-center gap-1.5 text-xs text-ink-faint'>
                  <Tag className='size-3 shrink-0' />
                  {sqlRelease(inst.Version, inst.Edition)}
                  {inst.Version && <span className='tabular-nums'>· {inst.Version}</span>}
                  {inst.Port && inst.Port !== "0" && <span>· port {inst.Port}</span>}
                </p>
                <p className='flex items-center gap-1.5 text-xs text-ink-faint'>
                  <Server className='size-3 shrink-0' />
                  Running on{" "}
                  <Link to={`/hosts/${inst.MachineID}`} className='font-mono text-ink-dim hover:text-ink underline underline-offset-2'>
                    {(inst.Clustered ? inst.OwnerNode || inst.machine.Hostname : inst.machine.Hostname)?.toLowerCase()}
                  </Link>
                </p>
                {inCluster && (
                  <p className='flex items-center gap-1.5 text-xs text-ink-faint'>
                    <Network className='size-3 shrink-0' />
                    {inst.Clustered ? "Failover cluster instance" : "Node"} in cluster{" "}
                    <span className='font-mono text-ink-dim'>{inst.machine.Cluster!.name.toLowerCase()}</span>
                  </p>
                )}
              </div>
            </div>
          </div>
          <div className='flex items-center gap-2 shrink-0 pt-1'>
            {running ? (
              <Button size='sm' variant='outline' disabled={busy || offline} onClick={() => setConfirmRestart(true)}>
                <RotateCw className='size-3' />
                Restart
              </Button>
            ) : (
              <Button size='sm' variant='outline' disabled={busy || offline} onClick={() => void run("start")}>
                <Play className='size-3' />
                Start
              </Button>
            )}
            {inst.Clustered && inCluster && (
              <Button size='sm' variant='outline' disabled={busy || offline} onClick={() => setMoving(true)}>
                <Shuffle className='size-3' />
                Migrate
              </Button>
            )}
            <Button size='sm' variant='danger' disabled title='Stopping SQL Server remotely is not implemented'>
              <Square className='size-3' />
              Stop
            </Button>
          </div>
        </div>
      ) : isLoading ? (
        <div className='flex items-start gap-4 animate-pulse'>
          <div className='mt-1 size-9 rounded-lg border border-rim bg-surface-high shrink-0' />
          <div className='space-y-3'>
            <div className='h-4 w-48 rounded bg-surface-high' />
            <div className='h-2.5 w-64 rounded bg-surface-high' />
          </div>
        </div>
      ) : (
        <p className='text-sm text-ink-faint'>This SQL Server instance is no longer reported by any host in this project.</p>
      )}

      {inst?.Monitored && inst.MonitorError && <SqlCredentialsHint error={inst.MonitorError} />}

      {group && activeProject && (
        <>
          <div className='flex items-center gap-1 border-b border-rim'>
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={cn(
                  "cursor-pointer px-4 py-2 text-xs font-medium transition-colors border-b-2 -mb-px",
                  current === t.id ? "border-primary text-primary" : "border-transparent text-ink-faint hover:text-ink",
                )}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className={cn(offline && "opacity-40 pointer-events-none select-none")}>
            {current === "overview" && (
              <Suspense fallback={<OverviewSkeleton />}>
                <SqlOverviewTab group={group} />
              </Suspense>
            )}
            {current === "databases" && <SqlDatabasesTab group={group} />}
            {current === "jobs" && <SqlJobsTab projectKey={activeProject.Key} group={group} />}
            {current === "queries" && <SqlActivityTab projectKey={activeProject.Key} group={group} />}
          </div>
        </>
      )}

      {activeProject && (
        <MoveInstanceModal projectKey={activeProject.Key} group={moving ? group ?? null : null} onClose={() => setMoving(false)} />
      )}

      <ConfirmModal
        open={confirmRestart}
        title='Restart SQL Server'
        description={`Restart ${group?.name.toLowerCase() ?? ""}? Every database on it is unavailable until it comes back. This will not alert unless it fails to come back within 5 minutes.`}
        confirmLabel='Restart'
        onClose={() => setConfirmRestart(false)}
        onConfirm={() => {
          setConfirmRestart(false);
          void run("restart");
        }}
      />
    </div>
  );
}
