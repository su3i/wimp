import { useState, useMemo } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  Activity, Plus, Trash2, AlertCircle, ExternalLink,
  ShieldCheck, ShieldAlert, HelpCircle,
} from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { ConfirmModal } from '@/components/ui/ConfirmModal'
import { RowMenu } from '@/components/ui/RowMenu'
import { useProjectStore } from '@/store/project'
import { monitorService } from '@/services/monitor.service'
import { prometheusService, type PromInstantResult } from '@/services/prometheus.service'
import { usePageTitle } from '@/utils/usePageTitle'
import { cn } from '@/utils/cn'
import type { Monitor } from '@/types'

const INTERVALS = [
  { label: '30 seconds', value: 30 },
  { label: '1 minute', value: 60 },
  { label: '2 minutes', value: 120 },
  { label: '5 minutes', value: 300 },
  { label: '10 minutes', value: 600 },
]

function fmtInterval(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  return `${Math.round(seconds / 60)}m`
}

function fmtUptime(ratio: number | null): string {
  if (ratio == null) return '--'
  return `${(ratio * 100).toFixed(2)}%`
}

function sslDaysRemaining(unixTs: number | null): number | null {
  if (unixTs == null) return null
  return Math.floor((unixTs - Date.now() / 1000) / 86400)
}

function isHttps(url: string) {
  return url.toLowerCase().startsWith('https://')
}

type ProbeStatus = 'up' | 'down' | 'unknown'

function StatusDot({ status }: { status: ProbeStatus }) {
  return (
    <span className={cn(
      'size-2 rounded-full shrink-0',
      status === 'up' ? 'bg-success' :
      status === 'down' ? 'bg-danger animate-pulse' : 'bg-ink-faint/40',
    )} />
  )
}

function SslBadge({ days }: { days: number | null }) {
  if (days == null) return <span className='text-xs text-ink-faint'>--</span>
  if (days < 0) return (
    <div className='flex items-center gap-1 text-danger'>
      <ShieldAlert className='size-3 shrink-0' />
      <span className='text-xs font-medium'>Expired</span>
    </div>
  )
  if (days <= 14) return (
    <div className='flex items-center gap-1 text-[#d29922]'>
      <ShieldAlert className='size-3 shrink-0' />
      <span className='text-xs font-medium'>{days}d</span>
    </div>
  )
  return (
    <div className='flex items-center gap-1 text-success'>
      <ShieldCheck className='size-3 shrink-0' />
      <span className='text-xs font-medium'>{days}d</span>
    </div>
  )
}

function TableSkeleton() {
  return (
    <div className='rounded-lg border border-rim overflow-hidden'>
      <div className='border-b border-rim bg-surface-alt px-4 py-2.5 flex gap-4'>
        {['w-32', 'w-16', 'w-12', 'w-14'].map((w, i) => (
          <div key={i} className={cn('h-2.5 rounded bg-surface-high animate-pulse', i === 0 ? 'flex-1' : w)} />
        ))}
      </div>
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className='flex items-center gap-4 px-4 py-3.5 border-b border-rim last:border-0 animate-pulse'>
          <div className='size-2 rounded-full bg-surface-high shrink-0' />
          <div className='flex-1 h-3 rounded bg-surface-high' />
          <div className='w-12 h-2.5 rounded bg-surface-high' />
          <div className='w-10 h-2.5 rounded bg-surface-high' />
          <div className='w-8 h-2.5 rounded bg-surface-high' />
          <div className='size-5 rounded bg-surface-high' />
        </div>
      ))}
    </div>
  )
}

function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <div className='flex flex-col items-center justify-center py-24 rounded-lg border border-rim bg-surface text-center'>
      <div className='mb-4 flex size-12 items-center justify-center rounded-xl border border-rim bg-surface-alt'>
        <Activity className='size-5 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No endpoints</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        Add a URL to start tracking uptime and response times.
      </p>
      <Button className='mt-5' size='sm' onClick={onAdd}>
        <Plus className='size-3.5' />
        Add Endpoint
      </Button>
    </div>
  )
}

function NoProjectSelected() {
  return (
    <div className='flex flex-col items-center justify-center py-24 text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface'>
        <Activity className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No project selected</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        Select a project from the sidebar to view its endpoints.
      </p>
    </div>
  )
}

interface MonitorFormProps {
  open: boolean
  initial?: Monitor
  onClose: () => void
  onSave: (url: string, intervalSeconds: number) => Promise<void>
  saving: boolean
}

function MonitorModal({ open, initial, onClose, onSave, saving }: MonitorFormProps) {
  const [url, setUrl] = useState(initial?.URL ?? '')
  const [interval, setInterval] = useState(initial?.IntervalSeconds ?? 60)

  function reset() {
    setUrl(initial?.URL ?? '')
    setInterval(initial?.IntervalSeconds ?? 60)
  }

  function handleClose() {
    reset()
    onClose()
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={initial ? 'Edit Endpoint' : 'Add Endpoint'}
    >
      <form
        onSubmit={async e => {
          e.preventDefault()
          await onSave(url.trim(), interval)
        }}
        className='space-y-4'
      >
        <Input
          label='URL'
          placeholder='https://example.com/health'
          value={url}
          onChange={e => setUrl(e.target.value)}
          autoFocus
        />
        <div className='space-y-1.5'>
          <label className='text-xs font-medium text-ink-dim'>Check interval</label>
          <select
            value={interval}
            onChange={e => setInterval(Number(e.target.value))}
            className='w-full rounded-md border border-rim bg-surface px-3 py-2 text-xs text-ink focus:outline-none focus:ring-1 focus:ring-primary cursor-pointer'
          >
            {INTERVALS.map(opt => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
        <div className='flex justify-end gap-2 pt-1'>
          <Button type='button' variant='outline' onClick={handleClose} disabled={saving}>
            Cancel
          </Button>
          <Button type='submit' loading={saving} disabled={!url.trim()}>
            {initial ? 'Save Changes' : 'Add Endpoint'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}

const TH = 'px-5 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'

export function Monitors() {
  usePageTitle('Uptime Monitor')
  const { activeProject } = useProjectStore()
  const queryClient = useQueryClient()

  const [addOpen, setAddOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Monitor | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Monitor | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const { data: monitors, isLoading, isError } = useQuery({
    queryKey: ['monitors', activeProject?.Key],
    enabled: !!activeProject,
    queryFn: async () => {
      const { data } = await monitorService.list(activeProject!.Key)
      return data.monitors ?? []
    },
  })

  const promEnabled = prometheusService.isConfigured()

  const monitorIds = useMemo(
    () => (monitors ?? []).map(m => String(m.ID)),
    [monitors],
  )

  const idFilter = monitorIds.length > 0 ? `monitor_id=~"${monitorIds.join('|')}"` : null

  const { data: probeSuccess } = useQuery({
    queryKey: ['probe-success', monitorIds],
    enabled: promEnabled && monitorIds.length > 0,
    refetchInterval: 30_000,
    queryFn: () => prometheusService.instant(`probe_success{${idFilter}}`),
  })

  const { data: probeUptime } = useQuery({
    queryKey: ['probe-uptime', monitorIds],
    enabled: promEnabled && monitorIds.length > 0,
    refetchInterval: 300_000,
    queryFn: () => prometheusService.instant(`avg_over_time(probe_success{${idFilter}}[30d])`),
  })

  const { data: probeSsl } = useQuery({
    queryKey: ['probe-ssl', monitorIds],
    enabled: promEnabled && monitorIds.length > 0,
    refetchInterval: 3_600_000,
    queryFn: () => prometheusService.instant(`probe_ssl_earliest_cert_expiry{${idFilter}}`),
  })

  function indexByMonitorId(results: PromInstantResult[] | undefined): Map<string, number> {
    const m = new Map<string, number>()
    for (const r of results ?? []) {
      const id = r.metric['monitor_id']
      const val = parseFloat(r.value[1])
      if (id && !isNaN(val)) m.set(id, val)
    }
    return m
  }

  const successMap = useMemo(() => indexByMonitorId(probeSuccess), [probeSuccess])
  const uptimeMap = useMemo(() => indexByMonitorId(probeUptime), [probeUptime])
  const sslMap = useMemo(() => indexByMonitorId(probeSsl), [probeSsl])

  function statusFor(id: number): ProbeStatus {
    if (!promEnabled || !successMap.has(String(id))) return 'unknown'
    return successMap.get(String(id)) === 1 ? 'up' : 'down'
  }

  async function handleSave(url: string, intervalSeconds: number) {
    const name = (() => { try { return new URL(url).hostname } catch { return url } })()
    setSaving(true)
    try {
      if (editTarget) {
        await monitorService.update(activeProject!.Key, editTarget.ID, { name, url, interval_seconds: intervalSeconds })
        toast.success('Endpoint updated.')
        setEditTarget(null)
      } else {
        await monitorService.create(activeProject!.Key, { name, url, interval_seconds: intervalSeconds })
        toast.success('Endpoint added.')
        setAddOpen(false)
      }
      queryClient.invalidateQueries({ queryKey: ['monitors', activeProject!.Key] })
    } catch {
      toast.error('Failed to save endpoint.')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await monitorService.delete(activeProject!.Key, deleteTarget.ID)
      queryClient.invalidateQueries({ queryKey: ['monitors', activeProject!.Key] })
      toast.success('Endpoint deleted.')
      setDeleteTarget(null)
    } catch {
      toast.error('Failed to delete endpoint.')
    } finally {
      setDeleting(false)
    }
  }

  if (!activeProject) return <NoProjectSelected />

  return (
    <div className='space-y-5'>
      {/* Header */}
      <div className='flex items-center justify-between'>
        <div>
          <h1 className='text-base font-semibold text-ink'>Uptime Monitor</h1>
          <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
        </div>
        <Button size='sm' onClick={() => setAddOpen(true)}>
          <Plus className='size-3.5' />
          Add Endpoint
        </Button>
      </div>

      {/* Prometheus not configured notice */}
      {!promEnabled && (
        <div className='flex items-start gap-2.5 rounded-lg border border-rim bg-surface px-4 py-3'>
          <HelpCircle className='size-3.5 text-ink-faint shrink-0 mt-0.5' />
          <p className='text-xs text-ink-dim'>
            Set <span className='font-mono text-ink'>VITE_PROMETHEUS_URL</span> to enable live status, response times, and uptime metrics.
            Configure Prometheus with a <span className='font-mono text-ink'>blackbox_http</span> job pointing at{' '}
            <span className='font-mono text-ink'>/prometheus/monitors</span>.
          </p>
        </div>
      )}

      {/* Table */}
      {isLoading ? (
        <TableSkeleton />
      ) : isError ? (
        <div className='flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger'>
          <AlertCircle className='size-4 shrink-0' />
          Failed to load endpoints.
        </div>
      ) : !monitors?.length ? (
        <EmptyState onAdd={() => setAddOpen(true)} />
      ) : (
        <div className='rounded-lg border border-rim overflow-hidden'>
          <div className='grid grid-cols-[1fr_150px_150px_150px_50px] border-b border-rim bg-surface-alt'>
            <div className={TH}>Endpoint</div>
            <div className={TH}>Uptime</div>
            <div className={TH}>SSL</div>
            <div className={TH}>Frequency</div>
            <div className='px-3 py-2.5' />
          </div>

          {monitors.map(m => {
            const status = statusFor(m.ID)
            const idStr = String(m.ID)
            const uptime = uptimeMap.get(idStr) ?? null
            const sslExpiry = isHttps(m.URL) ? (sslMap.get(idStr) ?? null) : null
            const sslDays = sslDaysRemaining(sslExpiry)

            return (
              <div
                key={m.ID}
                className='grid grid-cols-[1fr_150px_150px_150px_50px] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100'
              >
                <div className='flex items-center gap-3 px-4 py-3.5 min-w-0'>
                  <StatusDot status={status} />
                  <a
                    href={m.URL}
                    target='_blank'
                    rel='noopener noreferrer'
                    onClick={e => e.stopPropagation()}
                    className='inline-flex items-center gap-1 text-xs font-medium text-ink hover:text-primary transition-colors truncate'
                  >
                    {m.URL}
                    <ExternalLink className='size-3 shrink-0 opacity-50' />
                  </a>
                </div>

                <div className='px-4 py-3.5'>
                  <span className={cn(
                    'text-xs tabular-nums font-medium whitespace-nowrap',
                    uptime == null ? 'text-ink-faint' :
                    uptime >= 0.999 ? 'text-success' :
                    uptime >= 0.95 ? 'text-[#d29922]' : 'text-danger'
                  )}>
                    {fmtUptime(uptime)}
                  </span>
                </div>

                <div className='px-4 py-3.5'>
                  <SslBadge days={sslDays} />
                </div>

                <div className='px-4 py-3.5'>
                  <span className='text-xs text-ink-dim tabular-nums whitespace-nowrap'>
                    {fmtInterval(m.IntervalSeconds)}
                  </span>
                </div>

                <div className='flex items-center justify-center py-3 pr-2'>
                  <RowMenu
                    items={[
                      {
                        icon: Activity,
                        label: 'Edit',
                        onClick: () => setEditTarget(m),
                      },
                      { type: 'separator' },
                      {
                        icon: Trash2,
                        label: 'Delete',
                        variant: 'danger',
                        onClick: () => setDeleteTarget(m),
                      },
                    ]}
                  />
                </div>
              </div>
            )
          })}
        </div>
      )}

      <MonitorModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onSave={handleSave}
        saving={saving}
      />

      <MonitorModal
        open={!!editTarget}
        initial={editTarget ?? undefined}
        onClose={() => setEditTarget(null)}
        onSave={handleSave}
        saving={saving}
      />

      <ConfirmModal
        open={!!deleteTarget}
        title='Delete Endpoint'
        description='Delete this endpoint? Uptime checks will stop immediately.'
        confirmLabel='Delete'
        loading={deleting}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void handleDelete()}
      />
    </div>
  )
}
