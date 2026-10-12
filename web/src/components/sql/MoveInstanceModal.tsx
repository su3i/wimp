import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { sqlInstanceService } from '@/services/sqlInstance.service'
import { apiError, type SqlInstanceGroup } from '@/utils/sql'

// Migrates a failover cluster instance to another node. Used from the list's row menu and the
// detail page's header, so both offer the same flow. Only nodes that are up and not already
// running it are offered; the server checks the same again.
export function MoveInstanceModal({
  projectKey, group, onClose,
}: { projectKey: string; group: SqlInstanceGroup | null; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [node, setNode] = useState('')
  const [moving, setMoving] = useState(false)

  const inst = group?.current
  const owner = inst?.OwnerNode.toLowerCase() ?? ''
  const nodes = (inst?.machine.Cluster?.nodes ?? []).filter(n => n.name.toLowerCase() !== owner)
  const selected = nodes.find(n => n.name === node)

  function close() {
    setNode('')
    onClose()
  }

  async function move() {
    if (!group || !inst || !node) return
    setMoving(true)
    const pending = toast.loading(`Migrating ${group.name.toLowerCase()} to ${node.toLowerCase()}...`)
    try {
      await sqlInstanceService.move(projectKey, inst.MachineID, inst.Name, node)
      toast.success(`${group.name.toLowerCase()} is now running on ${node.toLowerCase()}.`, { id: pending })
      queryClient.invalidateQueries({ queryKey: ['project-sql-instances', projectKey] })
      close()
    } catch (err) {
      toast.error(apiError(err, 'Migration failed.'), { id: pending })
    } finally {
      setMoving(false)
    }
  }

  return (
    <Modal
      open={!!group}
      onClose={close}
      title='Migrate Instance'
      description={
        group
          ? `${group.name.toLowerCase()} is running on ${owner}. Its databases are unavailable while it comes online on the new node.`
          : ''
      }
    >
      <div className='space-y-4'>
        <div className='space-y-1.5'>
          <label htmlFor='move-node' className='block text-xs font-medium text-ink-dim'>Migrate to</label>
          <select
            id='move-node'
            value={node}
            onChange={e => setNode(e.target.value)}
            className='h-9 w-full rounded-md border border-rim bg-surface px-3 font-mono text-xs text-ink focus:border-primary focus:outline-none cursor-pointer'
          >
            <option value='' disabled>Select a node</option>
            {nodes.map(n => (
              <option key={n.name} value={n.name} disabled={n.state !== 'Up'}>
                {n.name.toLowerCase()}{n.state !== 'Up' ? ` (${n.state.toLowerCase()})` : ''}
              </option>
            ))}
          </select>
          {nodes.length === 0 && <p className='text-xs text-ink-faint'>The cluster has no other node to migrate it to.</p>}
        </div>
        <div className='flex justify-end gap-2'>
          <Button variant='outline' onClick={close} disabled={moving}>Cancel</Button>
          <Button loading={moving} disabled={!selected || selected.state !== 'Up'} onClick={() => void move()}>
            Migrate
          </Button>
        </div>
      </div>
    </Modal>
  )
}
