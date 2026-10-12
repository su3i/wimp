import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { AlertCircle, ChevronDown, ChevronRight, Database, Info, Play, RefreshCw, RotateCw, Shuffle, Square } from 'lucide-react'
import { RowMenu } from '@/components/ui/RowMenu'
import type { RowMenuItem } from '@/components/ui/RowMenu'
import { ConfirmModal } from '@/components/ui/ConfirmModal'
import { NodePickerModal } from '@/components/machine/ClusterTab'
import { sqlInstanceService } from '@/services/sqlInstance.service'
import { prometheusService } from '@/services/prometheus.service'
import { cn } from '@/utils/cn'
import { apiError, instanceLabel } from '@/utils/sql'
import type { Cluster, SqlInstance, SqlLiveKind, SqlState } from '@/types'

const stateTone: Record<SqlState, { dot: string; text: string }> = {
  Running: { dot: 'bg-success', text: 'text-success' },
  Stopped: { dot: 'bg-danger', text: 'text-danger' },
  Failed: { dot: 'bg-danger', text: 'text-danger' },
  Starting: { dot: 'bg-warning', text: 'text-warning' },
  Stopping: { dot: 'bg-warning', text: 'text-warning' },
  Unknown: { dot: 'bg-ink-faint', text: 'text-ink-faint' },
}

export function SqlStateBadge({ state, label }: { state: SqlState; label?: string }) {
  const tone = stateTone[state] ?? stateTone.Unknown
  return (
    <span className={cn('inline-flex items-center gap-2 text-xs', tone.text)}>
      <span className={cn('size-1.5 rounded-full shrink-0', tone.dot)} />
      {label ? `${label} ${state.toLowerCase()}` : state}
    </span>
  )
}

// Shown when the agent could not read DMVs. The agent's own error is quoted, since it names
// exactly what is missing: a failed login, or one permission.
export function SqlCredentialsHint({ error }: { error: string }) {
  return (
    <div className='flex items-start gap-2.5 rounded-lg border border-rim bg-surface px-4 py-3'>
      <Info className='size-3.5 text-ink-dim shrink-0 mt-0.5' />
      <div className='text-xs text-ink-dim leading-relaxed space-y-1.5'>
        <p>
          Wait stats, blocking, backups, jobs and AG health need read access to SQL Server. Set{' '}
          <span className='font-mono text-ink'>SQLMONITORUSERNAME</span> and{' '}
          <span className='font-mono text-ink'>SQLMONITORPASSWORD</span> in your deployment settings, or leave them empty
          and grant <span className='font-mono text-ink'>NT AUTHORITY\SYSTEM</span> the same access: VIEW SERVER STATE,
          VIEW ANY DEFINITION, and db_datareader in msdb.
        </p>
        <p className='font-mono text-[0.6875rem] text-ink-faint break-all'>{error}</p>
      </div>
    </div>
  )
}

// ── Prometheus stats ──────────────────────────────────────────────────────────

function promValue(v: string) {
  return v.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function hostMatcher(hostname: string) {
  return `hostname=~"(?i)${promValue(hostname.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))}"`
}

const STATS: { key: string; label: string; query: (sel: string, wimpSel: string) => string; fmt: (v: number) => string }[] = [
  { key: 'batch', label: 'Batch req/s', query: s => `sum(rate(windows_mssql_sqlstats_batch_requests{${s}}[5m]))`, fmt: v => v.toFixed(1) },
  { key: 'ple', label: 'Page life', query: s => `min(windows_mssql_bufman_page_life_expectancy_seconds{${s}})`, fmt: v => `${Math.round(v)}s` },
  { key: 'conn', label: 'Connections', query: s => `sum(windows_mssql_genstats_user_connections{${s}})`, fmt: v => String(Math.round(v)) },
  { key: 'grants', label: 'Grants pending', query: s => `sum(windows_mssql_memmgr_pending_memory_grants{${s}})`, fmt: v => String(Math.round(v)) },
  { key: 'blocked', label: 'Blocked', query: (_, w) => `sum(wimp_mssql_blocked_requests{${w}})`, fmt: v => String(Math.round(v)) },
  { key: 'longest', label: 'Longest request', query: (_, w) => `max(wimp_mssql_longest_request_seconds{${w}})`, fmt: v => `${Math.round(v)}s` },
]

function InstanceStats({ inst, hostname }: { inst: SqlInstance; hostname: string }) {
  // Metrics come from whichever node runs the instance, which for an FCI may not be this one.
  const statsHost = inst.Clustered && inst.OwnerNode ? inst.OwnerNode : hostname
  const sel = `${hostMatcher(statsHost)},mssql_instance="${promValue(inst.Name)}"`
  const wimpSel = `${hostMatcher(statsHost)},sql_instance="${promValue(inst.Name)}"`

  const { data } = useQuery({
    queryKey: ['sql-stats', statsHost, inst.Name],
    enabled: prometheusService.isConfigured() && inst.State === 'Running',
    refetchInterval: 15_000,
    queryFn: async () => {
      const results = await Promise.all(STATS.map(s => prometheusService.instant(s.query(sel, wimpSel)).catch(() => [])))
      const out: Record<string, number | undefined> = {}
      STATS.forEach((s, i) => {
        const v = Number(results[i][0]?.value[1])
        out[s.key] = Number.isFinite(v) ? v : undefined
      })
      return out
    },
  })

  if (!prometheusService.isConfigured() || inst.State !== 'Running') return null

  return (
    <div className='grid grid-cols-6 gap-px bg-rim border-t border-rim'>
      {STATS.map(s => (
        <div key={s.key} className='bg-surface px-4 py-2.5'>
          <p className='text-[0.625rem] uppercase tracking-widest text-ink-faint'>{s.label}</p>
          <p className='mt-1 font-mono text-sm text-ink'>
            {data?.[s.key] !== undefined ? s.fmt(data[s.key]!) : 'N/A'}
          </p>
        </div>
      ))}
    </div>
  )
}

// ── Live views ────────────────────────────────────────────────────────────────

const JOB_STATUS: Record<string, string> = { '0': 'Failed', '1': 'Succeeded', '2': 'Retry', '3': 'Canceled', '4': 'Running' }

const ms = (v: unknown) => (typeof v === 'number' ? `${(v / 1000).toFixed(1)}s` : '')
const date = (v: unknown) => (typeof v === 'string' && v ? new Date(v).toLocaleString() : '')
const text = (v: unknown) => (v === null || v === undefined ? '' : String(v))

const LIVE_COLUMNS: Record<SqlLiveKind, { key: string; label: string; fmt?: (v: unknown) => string; wide?: boolean }[]> = {
  blocking: [
    { key: 'session_id', label: 'Session' },
    { key: 'blocking_session_id', label: 'Blocked by' },
    { key: 'wait_ms', label: 'Waiting', fmt: ms },
    { key: 'database_name', label: 'Database' },
    { key: 'login_name', label: 'Login' },
    { key: 'blocker_login', label: 'Blocker login' },
    { key: 'blocker_host', label: 'Blocker host' },
    { key: 'sql_text', label: 'Statement', wide: true },
  ],
  requests: [
    { key: 'session_id', label: 'Session' },
    { key: 'status', label: 'Status' },
    { key: 'elapsed_ms', label: 'Elapsed', fmt: ms },
    { key: 'cpu_ms', label: 'CPU', fmt: ms },
    { key: 'wait_type', label: 'Wait' },
    { key: 'database_name', label: 'Database' },
    { key: 'login_name', label: 'Login' },
    { key: 'sql_text', label: 'Statement', wide: true },
  ],
  jobs: [
    { key: 'name', label: 'Job' },
    { key: 'enabled', label: 'Enabled', fmt: v => (v ? 'Yes' : 'No') },
    { key: 'last_run_status', label: 'Last outcome', fmt: v => JOB_STATUS[text(v)] ?? 'Never run' },
    { key: 'last_run_at', label: 'Last run', fmt: date },
    { key: 'next_run_at', label: 'Next run', fmt: date },
    { key: 'last_message', label: 'Message', wide: true },
  ],
}

const LIVE_TABS: { kind: SqlLiveKind; label: string }[] = [
  { kind: 'blocking', label: 'Blocking' },
  { kind: 'requests', label: 'Running requests' },
  { kind: 'jobs', label: 'Agent jobs' },
]

function LiveView({ projectKey, machineId, inst }: { projectKey: string; machineId: number; inst: SqlInstance }) {
  const [kind, setKind] = useState<SqlLiveKind>('blocking')
  const { data, isFetching, isError, error, refetch } = useQuery({
    queryKey: ['sql-live', projectKey, machineId, inst.Name, kind],
    retry: false,
    queryFn: async () => (await sqlInstanceService.live(projectKey, machineId, inst.Name, kind)).data.rows ?? [],
  })
  const cols = LIVE_COLUMNS[kind]

  return (
    <div className='border-t border-rim bg-canvas/40 px-4 py-3 space-y-3'>
      <div className='flex items-center gap-1.5'>
        {LIVE_TABS.map(t => (
          <button
            key={t.kind}
            type='button'
            onClick={() => setKind(t.kind)}
            className={cn(
              'px-3 py-1 rounded-md text-xs font-medium transition-colors cursor-pointer',
              kind === t.kind ? 'bg-primary/10 text-primary' : 'text-ink-faint hover:text-ink hover:bg-surface-high',
            )}
          >
            {t.label}
          </button>
        ))}
        <button
          type='button'
          onClick={() => void refetch()}
          className='ml-auto flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink cursor-pointer'
        >
          <RefreshCw className={cn('size-3', isFetching && 'animate-spin')} />
          Refresh
        </button>
      </div>

      {isError ? (
        <p className='flex items-center gap-2 text-xs text-danger'>
          <AlertCircle className='size-3.5 shrink-0' />
          {apiError(error, 'Query failed.')}
        </p>
      ) : !data ? (
        <div className='h-16 rounded-md bg-surface-high animate-pulse' />
      ) : data.length === 0 ? (
        <p className='text-xs text-ink-faint py-2'>
          {kind === 'blocking' ? 'Nothing is blocked right now.' : kind === 'requests' ? 'No user requests running.' : 'No SQL Agent jobs.'}
        </p>
      ) : (
        <div className='overflow-x-auto rounded-md border border-rim'>
          <table className='w-full text-xs'>
            <thead className='bg-surface-alt'>
              <tr>
                {cols.map(c => (
                  <th key={c.key} className='px-3 py-2 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint whitespace-nowrap'>
                    {c.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.map((row, i) => (
                <tr key={i} className='border-t border-rim'>
                  {cols.map(c => {
                    const v = (c.fmt ?? text)(row[c.key])
                    return (
                      <td
                        key={c.key}
                        title={c.wide ? v : undefined}
                        className={cn('px-3 py-2 text-ink-dim', c.wide ? 'font-mono max-w-[28rem] truncate' : 'whitespace-nowrap')}
                      >
                        {v}
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ── Tab ───────────────────────────────────────────────────────────────────────

export function InstancesTab({
  projectKey, machineId, hostname, cluster, isOffline,
}: { projectKey: string; machineId: number; hostname: string; cluster: Cluster | null; isOffline?: boolean }) {
  const [expanded, setExpanded] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [confirmRestart, setConfirmRestart] = useState<SqlInstance | null>(null)
  const [moving, setMoving] = useState<SqlInstance | null>(null)

  const { data: instances, isLoading, isError } = useQuery({
    queryKey: ['sql-instances', projectKey, machineId],
    refetchInterval: 5_000,
    queryFn: async () => (await sqlInstanceService.list(projectKey, machineId)).data.sql_instances ?? [],
  })

  async function run(inst: SqlInstance, cmd: 'start' | 'restart') {
    setBusy(inst.Name)
    const pending = toast.loading(`${cmd === 'start' ? 'Starting' : 'Restarting'} ${instanceLabel(inst)}...`)
    try {
      await sqlInstanceService.command(projectKey, machineId, inst.Name, cmd)
      toast.success(`${instanceLabel(inst)} ${cmd === 'start' ? 'started' : 'restarted'}.`, { id: pending })
    } catch (err) {
      toast.error(apiError(err, 'Command failed.'), { id: pending })
    } finally {
      setBusy(null)
    }
  }

  if (isLoading) return <div className='h-24 rounded-lg border border-rim bg-surface animate-pulse' />
  if (isError) {
    return (
      <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
        <AlertCircle className='size-4 shrink-0' />
        Failed to load SQL instances.
      </div>
    )
  }
  if (!instances?.length) {
    return (
      <div className='flex flex-col items-center justify-center py-16 rounded-lg border border-rim bg-surface text-center'>
        <Database className='size-5 text-ink-faint mb-3' />
        <p className='text-xs text-ink-faint'>No SQL Server instances found on this host.</p>
      </div>
    )
  }

  const monitorError = instances.find(i => i.Monitored && i.MonitorError)?.MonitorError

  return (
    <div className='space-y-3'>
      {monitorError && <SqlCredentialsHint error={monitorError} />}

      {instances.map(inst => {
        const open = expanded === inst.Name
        const running = inst.State === 'Running'
        const items: RowMenuItem[] = [
          running
            ? { icon: RotateCw, label: 'Restart', onClick: () => setConfirmRestart(inst) }
            : { icon: Play, label: 'Start', onClick: () => void run(inst, 'start') },
          { icon: Square, label: 'Stop', variant: 'danger', disabled: true, onClick: () => {} },
        ]
        if (inst.Clustered) {
          items.push({ type: 'separator' }, { icon: Shuffle, label: 'Move to node...', onClick: () => setMoving(inst) })
        }
        return (
          <div key={inst.ID} className={cn('rounded-lg border border-rim bg-surface overflow-hidden', busy === inst.Name && 'opacity-60')}>
            <div className='flex items-center gap-4 px-4 py-3.5'>
              <button
                type='button'
                onClick={() => setExpanded(open ? null : inst.Name)}
                className='flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high text-ink-faint hover:text-ink cursor-pointer'
                title={open ? 'Hide live view' : 'Show live view'}
              >
                {open ? <ChevronDown className='size-3' /> : <ChevronRight className='size-3' />}
              </button>
              <div className='min-w-0 flex-1'>
                <div className='flex items-center gap-3'>
                  <span className='font-mono text-xs text-ink'>{instanceLabel(inst)}</span>
                  <SqlStateBadge state={inst.State} />
                  {running && <SqlStateBadge state={inst.AgentState} label='Agent' />}
                </div>
                <p className='mt-1 text-[0.6875rem] text-ink-faint'>
                  {[inst.Edition, inst.Version && `v${inst.Version}`, inst.Port && inst.Port !== '0' && `port ${inst.Port}`]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
              </div>
              <div className='text-right text-[0.6875rem] text-ink-faint'>
                {inst.Clustered ? (
                  <>
                    <p>Failover cluster instance <span className='font-mono text-ink-dim'>{inst.VirtualServerName.toLowerCase()}</span></p>
                    {inst.OwnerNode && <p>running on <span className='font-mono text-ink-dim'>{inst.OwnerNode.toLowerCase()}</span></p>}
                  </>
                ) : (
                  <p>Standalone</p>
                )}
              </div>
              <RowMenu items={items} disabled={!!busy || !!isOffline} />
            </div>
            <InstanceStats inst={inst} hostname={hostname} />
            {open && <LiveView projectKey={projectKey} machineId={machineId} inst={inst} />}
          </div>
        )
      })}

      <ConfirmModal
        open={!!confirmRestart}
        title='Restart SQL Server'
        description={
          confirmRestart
            ? `Restart ${instanceLabel(confirmRestart)}? Every database on it is unavailable until it comes back. The outage this causes is expected, so it will not alert unless the instance fails to come back within 5 minutes.`
            : ''
        }
        confirmLabel='Restart'
        onClose={() => setConfirmRestart(null)}
        onConfirm={() => {
          if (confirmRestart) void run(confirmRestart, 'restart')
          setConfirmRestart(null)
        }}
      />

      <NodePickerModal
        open={!!moving}
        title={`Move ${moving ? instanceLabel(moving) : ''}`}
        description='Moves the clustered instance to another node. Its databases are unavailable while it comes online there.'
        cluster={cluster}
        current={moving?.OwnerNode ?? ''}
        confirmLabel='Move'
        onClose={() => setMoving(null)}
        onConfirm={async node => {
          try {
            await sqlInstanceService.move(projectKey, machineId, moving!.Name, node)
            toast.success(`${instanceLabel(moving!)} moved to ${node.toLowerCase()}.`)
          } catch (err) {
            toast.error(apiError(err, 'Move failed.'))
            throw err
          }
        }}
      />
    </div>
  )
}
