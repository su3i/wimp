import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Database, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { ConfirmModal } from '@/components/ui/ConfirmModal'
import { Modal } from '@/components/ui/Modal'
import { SqlStateBadge, SqlStatusBadge } from '@/components/sql/SqlBadges'
import { sqlInstanceService } from '@/services/sqlInstance.service'
import { cn } from '@/utils/cn'
import { usePaged } from '@/utils/usePaged'
import { Pagination } from '@/components/ui/Pagination'
import { apiError, instanceLabel } from '@/utils/sql'
import type { SqlInstanceWithMachine } from '@/types'

const TH = 'px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'

// Where an instance lives, as people name it: the FCI's network name, or the host.
function serverName(i: SqlInstanceWithMachine) {
  return (i.Clustered && i.VirtualServerName ? i.VirtualServerName : i.machine.Hostname ?? '').toLowerCase()
}

// A clustered instance has one row per node; the picker offers it once.
function identity(i: SqlInstanceWithMachine) {
  return i.Clustered ? `fci:${i.VirtualServerName}/${i.Name}` : `${i.MachineID}:${i.Name}`
}

function AddSqlInstanceModal({
  open, onClose, projectKey, appId, members, onAdded,
}: {
  open: boolean
  onClose: () => void
  projectKey: string
  appId: number
  members: SqlInstanceWithMachine[]
  onAdded: () => void
}) {
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [saving, setSaving] = useState(false)

  const { data: all } = useQuery({
    queryKey: ['project-sql-instances', projectKey],
    enabled: open,
    queryFn: async () => (await sqlInstanceService.listProject(projectKey)).data.sql_instances ?? [],
  })

  const taken = new Set(members.map(identity))
  const seen = new Set<string>()
  const choices = (all ?? []).filter(i => {
    const id = identity(i)
    if (taken.has(id) || seen.has(id)) return false
    seen.add(id)
    return true
  })

  async function save() {
    setSaving(true)
    try {
      await sqlInstanceService.addToApplication(projectKey, appId, [...selected])
      setSelected(new Set())
      onAdded()
      onClose()
    } catch (err) {
      toast.error(apiError(err, 'Failed to add SQL Server instances.'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title='Add SQL Server Instances' description='Instances discovered on hosts in this project.'>
      <div className='space-y-4'>
        {!all ? (
          <div className='h-24 rounded-md bg-surface-high animate-pulse' />
        ) : choices.length === 0 ? (
          <p className='text-xs text-ink-faint'>Every SQL Server instance in this project is already part of this application.</p>
        ) : (
          <div className='max-h-80 overflow-y-auto rounded-md border border-rim'>
            {choices.map(i => {
              const on = selected.has(i.ID)
              return (
                <button
                  key={i.ID}
                  type='button'
                  onClick={() => {
                    const next = new Set(selected)
                    if (on) next.delete(i.ID)
                    else next.add(i.ID)
                    setSelected(next)
                  }}
                  className={cn(
                    'flex w-full items-center gap-3 border-b border-rim px-3 py-2.5 text-left last:border-0 transition-colors cursor-pointer',
                    on ? 'bg-primary/5' : 'hover:bg-surface-alt',
                  )}
                >
                  <span className={cn('size-3.5 shrink-0 rounded border', on ? 'border-primary bg-primary' : 'border-rim-strong')} />
                  <span className='min-w-0 flex-1'>
                    <span className='block font-mono text-xs text-ink'>{instanceLabel(i)}</span>
                    <span className='block text-[0.6875rem] text-ink-faint'>
                      {serverName(i)}{i.Clustered ? ' (failover cluster instance)' : ''}
                    </span>
                  </span>
                  <SqlStateBadge state={i.State} />
                </button>
              )
            })}
          </div>
        )}
        <div className='flex justify-end gap-2'>
          <Button variant='outline' onClick={onClose} disabled={saving}>Cancel</Button>
          <Button loading={saving} disabled={selected.size === 0} onClick={() => void save()}>Add</Button>
        </div>
      </div>
    </Modal>
  )
}

// The SQL Server instances that are part of an application, next to its app pools. Shown
// only when the project has SQL Server at all, or the application already has members.
export function SqlMembers({
  projectKey, appId, members, onChange,
}: { projectKey: string; appId: number; members: SqlInstanceWithMachine[]; onChange: () => void }) {
  const navigate = useNavigate()
  const [adding, setAdding] = useState(false)
  const [removing, setRemoving] = useState<SqlInstanceWithMachine | null>(null)

  const paged = usePaged(members)
  const { data: projectHasSql } = useQuery({
    queryKey: ['project-sql-instances', projectKey],
    enabled: members.length === 0,
    queryFn: async () => (await sqlInstanceService.listProject(projectKey)).data.sql_instances ?? [],
    select: d => d.length > 0,
  })
  if (members.length === 0 && !projectHasSql) return null

  const cols = 'grid-cols-[2fr_1.5fr_1.2fr_auto]'

  return (
    // No overflow-hidden: it would clip the status tooltips. Header and last row carry the rounding.
    <div className='rounded-lg border border-rim'>
      <div className='flex items-center justify-between rounded-t-lg border-b border-rim bg-surface-alt px-4 py-2.5'>
        <p className='flex items-center gap-2 text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
          <Database className='size-3' />
          SQL Server Instances
        </p>
        <Button size='sm' variant='outline' onClick={() => setAdding(true)}>
          <Plus className='size-3' />
          Add SQL Server Instance
        </Button>
      </div>

      {members.length === 0 ? (
        <p className='px-4 py-6 text-center text-xs text-ink-faint'>No SQL Server instances in this application yet.</p>
      ) : (
        <>
          <div className={cn('grid border-b border-rim bg-surface', cols)}>
            <div className={TH}>Instance</div>
            <div className={TH}>Server</div>
            <div className={TH}>Status</div>
            <div className='px-4 py-2.5' />
          </div>
          {paged.items.map(i => {
            return (
              <div
                key={i.ID}
                onClick={() => navigate(`/sql-instances/${i.ID}`)}
                className={cn('grid items-center border-b border-rim last:border-0 last:rounded-b-lg bg-surface hover:bg-surface-alt cursor-pointer', cols)}
              >
                <div className='px-4 py-3 font-mono text-xs text-ink'>{instanceLabel(i)}</div>
                <div className='px-4 py-3 text-xs text-ink-dim'>
                  <span className='font-mono'>{serverName(i)}</span>
                  {i.Clustered && i.OwnerNode && <span className='text-ink-faint'> on {i.OwnerNode.toLowerCase()}</span>}
                </div>
                <div className='px-4 py-3'><SqlStatusBadge instance={i} /></div>
                <div className='px-3 py-2'>
                  <button
                    type='button'
                    title='Remove from application'
                    onClick={e => {
                      e.stopPropagation()
                      setRemoving(i)
                    }}
                    className='flex size-7 items-center justify-center rounded-md text-ink-faint hover:bg-surface-high hover:text-danger cursor-pointer'
                  >
                    <Trash2 className='size-3.5' />
                  </button>
                </div>
              </div>
            )
          })}
          {paged.totalPages > 1 && (
            <div className='rounded-b-lg bg-surface px-4 py-2.5'>
              <Pagination page={paged.page} totalPages={paged.totalPages} total={paged.total} pageSize={paged.pageSize} onPageChange={paged.setPage} />
            </div>
          )}
        </>
      )}

      <AddSqlInstanceModal
        open={adding}
        onClose={() => setAdding(false)}
        projectKey={projectKey}
        appId={appId}
        members={members}
        onAdded={onChange}
      />

      <ConfirmModal
        open={!!removing}
        title='Remove SQL Server Instance'
        description={removing ? `Remove ${instanceLabel(removing)} on ${serverName(removing)} from this application? The instance itself is not touched.` : ''}
        confirmLabel='Remove'
        onClose={() => setRemoving(null)}
        onConfirm={async () => {
          if (!removing) return
          try {
            await sqlInstanceService.removeFromApplication(projectKey, appId, removing.ID)
            onChange()
          } catch (err) {
            toast.error(apiError(err, 'Failed to remove SQL Server instance.'))
          }
          setRemoving(null)
        }}
      />
    </div>
  )
}
