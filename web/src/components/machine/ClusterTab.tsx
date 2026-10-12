import { useState } from 'react'
import { toast } from 'sonner'
import { ArrowRightLeft, Network, Server } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { sqlInstanceService } from '@/services/sqlInstance.service'
import { cn } from '@/utils/cn'
import { apiError } from '@/utils/sql'
import type { Cluster } from '@/types'

const TH = 'px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'

function nodeTone(state: string) {
  if (state === 'Up') return { dot: 'bg-success', text: 'text-success' }
  if (state === 'Down') return { dot: 'bg-danger', text: 'text-danger' }
  return { dot: 'bg-warning', text: 'text-warning' }
}

function StateDot({ state }: { state: string }) {
  const tone = nodeTone(state)
  return (
    <span className={cn('inline-flex items-center gap-2 text-xs', tone.text)}>
      <span className={cn('size-1.5 rounded-full shrink-0', tone.dot)} />
      {state}
    </span>
  )
}

// Picks the node a clustered instance or availability group moves to. Only nodes that are up
// and not already running it are offered; the server checks the same again.
export function NodePickerModal({
  open, title, description, cluster, current, confirmLabel, onClose, onConfirm,
}: {
  open: boolean
  title: string
  description: string
  cluster: Cluster | null
  current: string
  confirmLabel: string
  onClose: () => void
  onConfirm: (node: string) => Promise<void>
}) {
  const [node, setNode] = useState('')
  const [busy, setBusy] = useState(false)
  const candidates = (cluster?.nodes ?? []).filter(
    n => n.state === 'Up' && n.name.toLowerCase() !== current.toLowerCase(),
  )

  async function confirm() {
    setBusy(true)
    try {
      await onConfirm(node)
      setNode('')
      onClose()
    } catch {
      // onConfirm has already reported it; the modal stays open to try another node.
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={title} description={description}>
      <div className='space-y-4'>
        {candidates.length === 0 ? (
          <p className='text-xs text-ink-faint'>No other node is up to take it.</p>
        ) : (
          <div className='grid grid-cols-2 gap-2'>
            {candidates.map(n => (
              <button
                key={n.name}
                type='button'
                onClick={() => setNode(n.name)}
                className={cn(
                  'flex items-center gap-2 rounded-md border px-3 py-2.5 text-left font-mono text-xs transition-colors cursor-pointer',
                  node === n.name ? 'border-primary/60 bg-primary/5 text-ink' : 'border-rim bg-surface text-ink-dim hover:border-rim-strong',
                )}
              >
                <Server className='size-3 shrink-0' />
                {n.name.toLowerCase()}
              </button>
            ))}
          </div>
        )}
        <div className='flex justify-end gap-2'>
          <Button variant='outline' onClick={onClose} disabled={busy}>Cancel</Button>
          <Button loading={busy} disabled={!node} onClick={() => void confirm()}>{confirmLabel}</Button>
        </div>
      </div>
    </Modal>
  )
}

export function ClusterTab({
  projectKey, machineId, cluster, isOffline,
}: { projectKey: string; machineId: number; cluster: Cluster | null; isOffline?: boolean }) {
  const [failoverAg, setFailoverAg] = useState<{ name: string; primary: string } | null>(null)

  if (!cluster) {
    return (
      <div className='flex flex-col items-center justify-center py-16 rounded-lg border border-rim bg-surface text-center'>
        <Network className='size-5 text-ink-faint mb-3' />
        <p className='text-xs text-ink-faint'>This host is not a failover cluster node.</p>
      </div>
    )
  }

  const nodes = cluster.nodes ?? []
  const ags = cluster.availability_groups ?? []

  return (
    <div className='space-y-5'>
      <p className='text-xs text-ink-faint'>
        Failover cluster <span className='font-mono text-ink'>{cluster.name.toLowerCase()}</span>, as seen from this node.
      </p>

      <div className='rounded-lg border border-rim overflow-hidden'>
        <div className='grid grid-cols-[2fr_1fr] border-b border-rim bg-surface-alt'>
          <div className={TH}>Node</div>
          <div className={TH}>State</div>
        </div>
        {nodes.map(n => (
          <div key={n.name} className='grid grid-cols-[2fr_1fr] items-center border-b border-rim last:border-0'>
            <div className='px-4 py-3 font-mono text-xs text-ink'>{n.name.toLowerCase()}</div>
            <div className='px-4 py-3'><StateDot state={n.state} /></div>
          </div>
        ))}
      </div>

      {ags.length > 0 && (
        <div className='rounded-lg border border-rim overflow-hidden'>
          <div className='grid grid-cols-[2fr_1.5fr_1fr_auto] border-b border-rim bg-surface-alt'>
            <div className={TH}>Availability Group</div>
            <div className={TH}>Primary</div>
            <div className={TH}>State</div>
            <div className='px-4 py-2.5' />
          </div>
          {ags.map(ag => (
            <div key={ag.name} className='grid grid-cols-[2fr_1.5fr_1fr_auto] items-center border-b border-rim last:border-0'>
              <div className='px-4 py-3 font-mono text-xs text-ink'>{ag.name}</div>
              <div className='px-4 py-3 font-mono text-xs text-ink-dim'>{ag.primary_node.toLowerCase()}</div>
              <div className='px-4 py-3'><StateDot state={ag.state === 'Online' ? 'Up' : ag.state} /></div>
              <div className='px-3 py-2'>
                <Button
                  size='sm'
                  variant='outline'
                  disabled={isOffline}
                  onClick={() => setFailoverAg({ name: ag.name, primary: ag.primary_node })}
                >
                  <ArrowRightLeft className='size-3' />
                  Fail over
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <NodePickerModal
        open={!!failoverAg}
        title={`Fail over ${failoverAg?.name ?? ''}`}
        description='A planned failover with no data loss. The target replica must use synchronous commit with every database SYNCHRONIZED, or it refuses. Forced failover is not offered.'
        cluster={cluster}
        current={failoverAg?.primary ?? ''}
        confirmLabel='Fail over'
        onClose={() => setFailoverAg(null)}
        onConfirm={async node => {
          try {
            await sqlInstanceService.failoverAg(projectKey, machineId, failoverAg!.name, node)
            toast.success(`${failoverAg!.name} failed over to ${node.toLowerCase()}.`)
          } catch (err) {
            toast.error(apiError(err, 'Failover failed.'))
            throw err
          }
        }}
      />
    </div>
  )
}
