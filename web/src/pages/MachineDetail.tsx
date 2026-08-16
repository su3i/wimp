import { useState, lazy, Suspense } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, Info, Monitor, Network, Power, RotateCw, Server } from 'lucide-react'
import { useProjectStore } from '@/store/project'
import { machineService } from '@/services/machine.service'
import { AppPoolsTab } from '@/components/machine/AppPoolsTab'
import { SitesTab } from '@/components/machine/SitesTab'
import { Button } from '@/components/ui/Button'
import { ConfirmModal } from '@/components/ui/ConfirmModal'
import { cn } from '@/utils/cn'
import { usePageTitle } from '@/utils/usePageTitle'
import { useActionCooldown } from '@/utils/useActionCooldown'
import type { MachineStatus } from '@/types'

// MetricsTab pulls in recharts (a large dependency) - split into its own chunk so the
// page shell (header, tab bar, actions) can paint before it's fetched.
const MetricsTab = lazy(() => import('@/components/machine/MetricsTab').then(m => ({ default: m.MetricsTab })))

function filterIPv4(ips: string[]) {
  return (ips ?? []).filter(ip => /^\d{1,3}(\.\d{1,3}){3}$/.test(ip))
}

function MetricsTabSkeleton() {
  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-3 gap-3'>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='h-24 rounded-lg border border-rim bg-surface animate-pulse' />
        ))}
      </div>
      <div className='grid grid-cols-2 gap-4'>
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className='h-48 rounded-lg border border-rim bg-surface animate-pulse' />
        ))}
      </div>
    </div>
  )
}

const machineStatusCfg: Record<MachineStatus, { dot: string; label: string; text: string; pill: string }> = {
  online:  { dot: 'bg-success', label: 'Online',  text: 'text-success', pill: 'bg-success/10 text-success'  },
  offline: { dot: 'bg-danger',  label: 'Offline', text: 'text-danger',  pill: 'bg-danger/10 text-danger'   },
  pending: { dot: 'bg-warning', label: 'Pending', text: 'text-warning', pill: 'bg-warning/10 text-warning' },
  reassigned: { dot: 'bg-ink-faint', label: 'Reassigned', text: 'text-ink-faint', pill: 'bg-surface-high text-ink-faint' },
}

type Tab = 'metrics' | 'pools' | 'sites'

const TABS: { id: Tab; label: string }[] = [
  { id: 'metrics', label: 'Overview' },
  { id: 'pools',   label: 'App Pools' },
  { id: 'sites',   label: 'Sites' },
]

export function MachineDetail() {
  const { machineId } = useParams<{ machineId: string }>()
  const navigate = useNavigate()
  const { activeProject } = useProjectStore()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<Tab>('metrics')
  const [confirmAction, setConfirmAction] = useState<'shutdown' | 'restart' | null>(null)
  const [actingOnMachine, setActingOnMachine] = useState(false)
  const cooldown = useActionCooldown()

  const numericId = Number(machineId)

  const { data: machines } = useQuery({
    queryKey: ['machines', activeProject?.Key],
    enabled: !!activeProject,
    refetchInterval: 5_000,
    queryFn: async () => {
      const { data } = await machineService.list(activeProject!.Key, { per_page: 100 })
      return data.machines ?? []
    },
  })

  const machine = machines?.find(m => m.ID === numericId)
  usePageTitle(machine?.Hostname?.toLowerCase() ?? undefined)
  const machineCfg = machine ? machineStatusCfg[machine.Status] : null
  const ipv4s = machine ? filterIPv4(machine.IPs) : []
  const isOffline = machine?.Status === 'offline'
  // Reassigned rows are kept for their history but describe a machine that now reports
  // elsewhere, so everything below the header is a frozen snapshot rather than live state.
  const isReassigned = machine?.Status === 'reassigned'
  const cooling = cooldown.isCooling(numericId)

  async function handleMachineCommand() {
    if (!confirmAction || !activeProject) return
    setActingOnMachine(true)
    cooldown.start(numericId)
    try {
      await machineService.command(activeProject.Key, numericId, confirmAction)
      queryClient.invalidateQueries({ queryKey: ['machines', activeProject.Key] })
      setConfirmAction(null)
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Command failed. Please try again.'
      toast.error(msg)
    } finally {
      setActingOnMachine(false)
    }
  }

  return (
    <div className="space-y-5">
      {/* Back */}
      <button
        onClick={() => navigate('/hosts')}
        className="cursor-pointer flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink transition-colors"
      >
        <ArrowLeft className="size-3.5" />
        Hosts
      </button>

      {/* Machine header */}
      {machine ? (
        <div className="flex items-start justify-between gap-6">
          <div className="flex items-start gap-4">
            <div className="mt-1 flex size-9 shrink-0 items-center justify-center rounded-lg border border-rim bg-surface-alt">
              <Server className="size-4 text-ink-faint" />
            </div>
            <div>
              <div className="flex items-center gap-2.5 mb-3">
                <h1 className="font-mono text-base font-semibold text-ink">{machine.Hostname?.toLowerCase() || 'Awaiting connection...'}</h1>
                {machineCfg && (
                  <span className={cn('inline-flex items-center gap-1.5 text-xs font-medium', machineCfg.text)}>
                    <span className={cn('size-1.5 rounded-full shrink-0', machineCfg.dot)} />
                    {machineCfg.label}
                  </span>
                )}
              </div>
              <div className="space-y-2">
                {ipv4s.length > 0 && (
                  <p className="flex items-center gap-1.5 text-xs text-ink-faint">
                    <Network className="size-3 shrink-0" />
                    {ipv4s.join(' · ')}
                  </p>
                )}
                {machine.WindowsVersion && (
                  <p className="flex items-center gap-1.5 text-xs text-ink-faint">
                    <Monitor className="size-3 shrink-0" />
                    {machine.WindowsVersion}
                  </p>
                )}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0 pt-1">
            <Button
              size="sm"
              variant="outline"
              disabled={actingOnMachine || isOffline || cooling}
              onClick={() => setConfirmAction('restart')}
            >
              <RotateCw className="size-3" />
              Restart
            </Button>
            <Button
              size="sm"
              variant="danger"
              disabled
              title="Remote shutdown is temporarily disabled"
              onClick={() => setConfirmAction('shutdown')}
            >
              <Power className="size-3" />
              Shutdown
            </Button>
          </div>
        </div>
      ) : (
        <div className="flex items-start gap-4 animate-pulse">
          <div className="mt-1 size-9 rounded-lg border border-rim bg-surface-high shrink-0" />
          <div>
            <div className="flex items-center gap-2.5 mb-3">
              <div className="h-4 w-48 rounded bg-surface-high" />
              <div className="h-5 w-14 rounded-full bg-surface-high" />
            </div>
            <div className="space-y-1.5">
              <div className="h-2.5 w-28 rounded bg-surface-high" />
              <div className="h-2.5 w-20 rounded bg-surface-high" />
            </div>
          </div>
        </div>
      )}

      {isReassigned && (
        <div className="flex items-start gap-2.5 rounded-lg border border-rim bg-surface-alt px-4 py-3 text-xs text-ink-dim">
          <Info className="size-4 shrink-0 text-ink-faint" />
          <p>
            This host was re-bootstrapped and now reports to a different host entry
            {machine?.SupersededByID ? ` (host #${machine.SupersededByID})` : ''}. Everything below
            is the last state recorded before it moved, kept for reference. It will not come back
            online here, and it no longer reports metrics or logs to this project.
          </p>
        </div>
      )}

      {/* Tabs */}
      <div className="flex items-center gap-1 border-b border-rim">
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              'cursor-pointer px-4 py-2 text-xs font-medium transition-colors border-b-2 -mb-px',
              activeTab === tab.id
                ? 'border-primary text-primary'
                : 'border-transparent text-ink-faint hover:text-ink',
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className={cn((isOffline || isReassigned) && 'opacity-40 pointer-events-none select-none')}>
        {activeTab === 'metrics' && (
          <Suspense fallback={<MetricsTabSkeleton />}>
            <MetricsTab machineId={numericId} />
          </Suspense>
        )}
        {activeTab === 'pools' && activeProject && (
          <AppPoolsTab projectKey={activeProject.Key} machineId={numericId} isOffline={isOffline} />
        )}
        {activeTab === 'sites' && activeProject && (
          <SitesTab projectKey={activeProject.Key} machineId={numericId} isOffline={isOffline} />
        )}
      </div>

      <ConfirmModal
        open={!!confirmAction}
        title={confirmAction === 'shutdown' ? 'Shutdown Host' : 'Restart Host'}
        description={
          confirmAction === 'shutdown'
            ? `Are you sure you want to shut down "${machine?.Hostname || 'this host'}"? It will go offline until someone powers it back on.`
            : `Are you sure you want to restart "${machine?.Hostname || 'this host'}"? It will be briefly unreachable.`
        }
        confirmLabel={confirmAction === 'shutdown' ? 'Shutdown' : 'Restart'}
        loading={actingOnMachine}
        onClose={() => setConfirmAction(null)}
        onConfirm={() => void handleMachineCommand()}
      />
    </div>
  )
}
