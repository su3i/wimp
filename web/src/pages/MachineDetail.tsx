import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, Server, Clock, Power, RotateCw } from 'lucide-react'
import { useProjectStore } from '@/store/project'
import { machineService } from '@/services/machine.service'
import { AppPoolsTab } from '@/components/machine/AppPoolsTab'
import { SitesTab } from '@/components/machine/SitesTab'
import { MetricsTab } from '@/components/machine/MetricsTab'
import { Button } from '@/components/ui/Button'
import { ConfirmModal } from '@/components/ui/ConfirmModal'
import { cn } from '@/utils/cn'
import { usePageTitle } from '@/utils/usePageTitle'
import type { MachineStatus } from '@/types'

// ── Helpers ───────────────────────────────────────────────────────────────────

function filterIPv4(ips: string[]) {
  return (ips ?? []).filter(ip => /^\d{1,3}(\.\d{1,3}){3}$/.test(ip))
}

function formatLastPing(lastSeenAt: string | null) {
  if (!lastSeenAt) return 'Never'
  const diff = Date.now() - new Date(lastSeenAt).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'Just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}

const machineStatusCfg: Record<MachineStatus, { dot: string; text: string; label: string }> = {
  online:  { dot: 'bg-success', text: 'text-success', label: 'Online'  },
  offline: { dot: 'bg-danger',  text: 'text-danger',  label: 'Offline' },
  pending: { dot: 'bg-warning', text: 'text-warning', label: 'Pending' },
}

// ── Page ──────────────────────────────────────────────────────────────────────

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

  const numericId = Number(machineId)

  const { data: machines } = useQuery({
    queryKey: ['machines', activeProject?.Key],
    enabled: !!activeProject,
    refetchInterval: 5_000,
    queryFn: async () => {
      const { data } = await machineService.list(activeProject!.Key)
      return data.machines ?? []
    },
  })

  const machine = machines?.find(m => m.ID === numericId)
  usePageTitle(machine?.Hostname?.toLowerCase() ?? undefined)
  const machineCfg = machine ? machineStatusCfg[machine.Status] : null
  const ipv4s = machine ? filterIPv4(machine.IPs) : []

  async function handleMachineCommand() {
    if (!confirmAction || !activeProject) return
    setActingOnMachine(true)
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
        onClick={() => navigate('/machines')}
        className="cursor-pointer flex items-center gap-1.5 text-xs text-ink-faint hover:text-ink transition-colors"
      >
        <ArrowLeft className="size-3.5" />
        Hosts
      </button>

      {/* Machine header card */}
      <div className="rounded-lg border border-rim bg-surface px-5 py-4">
        {machine ? (
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3.5 min-w-0">
              <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-rim bg-surface-high">
                <Server className="size-4 text-ink-faint" />
              </div>
              <div className="min-w-0">
                <h1 className="font-mono text-sm font-semibold text-ink truncate">{machine.Hostname?.toLowerCase() || 'Awaiting connection...'}</h1>
                <div className="flex items-center gap-3 mt-0.5 text-[0.6875rem] text-ink-faint">
                  {ipv4s.length > 0 && <span className="font-mono">{ipv4s.join(' / ')}</span>}
                  {ipv4s.length > 0 && <span>·</span>}
                  <span className="flex items-center gap-1">
                    <Clock className="size-3" />
                    {formatLastPing(machine.LastSeenAt)}
                  </span>
                </div>
              </div>
            </div>
            <div className="flex items-center gap-4 shrink-0">
              {machineCfg && (
                <div className="flex items-center gap-2">
                  <span className={cn('size-2 rounded-full', machineCfg.dot)} />
                  <span className={cn('text-xs font-medium', machineCfg.text)}>{machineCfg.label}</span>
                </div>
              )}
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={actingOnMachine}
                  onClick={() => setConfirmAction('restart')}
                >
                  <RotateCw className="size-3" />
                  Restart
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  disabled={actingOnMachine}
                  onClick={() => setConfirmAction('shutdown')}
                >
                  <Power className="size-3" />
                  Shutdown
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-3.5 animate-pulse">
            <div className="size-9 rounded-lg border border-rim bg-surface-high" />
            <div className="space-y-1.5">
              <div className="h-3.5 w-48 rounded bg-surface-high" />
              <div className="h-2.5 w-32 rounded bg-surface-high" />
            </div>
          </div>
        )}
      </div>

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
      {activeTab === 'metrics' && (
        <MetricsTab machineId={numericId} />
      )}
      {activeTab === 'pools' && activeProject && (
        <AppPoolsTab projectKey={activeProject.Key} machineId={numericId} />
      )}
      {activeTab === 'sites' && activeProject && (
        <SitesTab projectKey={activeProject.Key} machineId={numericId} />
      )}

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
