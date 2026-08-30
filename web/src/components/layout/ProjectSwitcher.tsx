import { useState, useRef, useEffect } from 'react'
import { createPortal } from 'react-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Layers, Check, Plus, X, Trash2, ChevronDown } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/utils/cn'
import { useProjectStore } from '@/store/project'
import { projectService } from '@/services/project.service'
import { CreateProjectModal } from '@/components/projects/CreateProjectModal'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import type { Project } from '@/types'

function DeleteProjectModal({
  project,
  onClose,
  onDeleted,
}: {
  project: Project
  onClose: () => void
  onDeleted: () => void
}) {
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!project) setConfirm('')
  }, [project])

  async function handleDelete() {
    setLoading(true)
    try {
      await projectService.delete(project.Key)
      onDeleted()
      onClose()
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Failed to delete project.'
      toast.error(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal open={!!project} onClose={onClose} title='Delete Project'>
      <div className='space-y-4'>
        <p className='text-sm text-ink-dim'>
          This will permanently delete{' '}
          <span className='font-semibold text-ink'>{project.Name}</span> and all its
          hosts, applications, and data. This action cannot be undone.
        </p>
        <Input
          label='Type DELETE to confirm'
          placeholder='DELETE'
          value={confirm}
          onChange={e => setConfirm(e.target.value)}
          autoComplete='off'
        />
        <div className='flex justify-end gap-2 pt-1'>
          <Button variant='outline' onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button
            variant='danger'
            disabled={confirm !== 'DELETE' || loading}
            loading={loading}
            onClick={() => void handleDelete()}
          >
            Delete Project
          </Button>
        </div>
      </div>
    </Modal>
  )
}

// Mirrors project.DefaultKey in internal/domain/project/enum.go. The default project is
// where the switcher lands a user whose current project has just been deleted, so it must
// always exist - the API refuses to delete it and the UI does not offer to.
const DEFAULT_PROJECT_KEY = 'default'

export function ProjectSwitcher({ expanded = false }: { expanded?: boolean }) {
  const [open, setOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null)
  const [pos, setPos] = useState({ top: 0, left: 0 })
  const buttonRef = useRef<HTMLButtonElement>(null)
  const queryClient = useQueryClient()

  const navigate = useNavigate()
  const { activeProject, setActiveProject, clearActiveProject } = useProjectStore()

  const { data: projects } = useQuery({
    queryKey: ['projects'],
    queryFn: async () => {
      const { data } = await projectService.list()
      return data.projects ?? []
    },
    staleTime: 60_000,
  })

  useEffect(() => {
    if (!activeProject && projects?.length) {
      setActiveProject(projects[0])
    }
  }, [projects, activeProject, setActiveProject])

  function handleToggle() {
    if (!buttonRef.current) return
    const rect = buttonRef.current.getBoundingClientRect()
    if (expanded) {
      setPos({ top: rect.bottom + 4, left: rect.left - 8 })
    } else {
      setPos({ top: rect.top, left: rect.right + 8 })
    }
    setOpen(v => !v)
  }

  useEffect(() => {
    if (!open) return
    function close() { setOpen(false) }
    function handleKey(e: KeyboardEvent) { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', handleKey)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', handleKey)
    }
  }, [open])

  function handleDeleted(project: Project) {
    queryClient.invalidateQueries({ queryKey: ['projects'] })
    queryClient.removeQueries({ predicate: q => q.queryKey[0] !== 'projects' })

    if (activeProject?.ID !== project.ID) return

    // Land on the default project explicitly rather than clearing and letting the
    // auto-select effect pick projects[0] - that is whatever order the API returned, so
    // the user would end up somewhere arbitrary. Falling back to a clear covers the
    // case where the list has not refetched yet; the effect then picks it up.
    const fallback = projects?.find(p => p.Key === DEFAULT_PROJECT_KEY && p.ID !== project.ID)
    if (fallback) {
      setActiveProject(fallback)
    } else {
      clearActiveProject()
    }
    navigate('/')
  }

  return (
    <>
      <button
        ref={buttonRef}
        type='button'
        onClick={handleToggle}
        title={activeProject?.Name ?? 'Select project'}
        className={cn(
          'cursor-pointer w-full h-8 flex items-center rounded-md transition-colors',
          expanded ? 'gap-2 px-2' : 'justify-center',
          'text-ink-dim hover:text-ink hover:bg-surface-high',
          open && 'bg-surface-high text-ink',
        )}
      >
        <Layers className='size-4 shrink-0 text-ink-faint' />
        {expanded && (
          <>
            <span className='flex-1 text-xs font-medium truncate text-left'>
              {activeProject?.Name ?? 'Select project'}
            </span>
            <ChevronDown className={cn('size-3.5 shrink-0 text-ink-faint transition-transform duration-150', open && 'rotate-180')} />
          </>
        )}
      </button>

      {open && createPortal(
        <div
          onMouseDown={e => e.stopPropagation()}
          style={{ position: 'fixed', top: pos.top, left: pos.left }}
          className={cn(
            'z-[9999] rounded-md border border-rim bg-surface shadow-[0_8px_28px_rgba(1,4,9,0.85)] overflow-hidden',
            expanded ? 'w-52' : 'w-64',
          )}
        >
          <div className='flex items-center justify-between border-b border-rim px-3.5 py-2.5'>
            <span className='text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'>
              Projects
            </span>
            <button
              type='button'
              onClick={() => setOpen(false)}
              className='cursor-pointer text-ink-faint hover:text-ink transition-colors'
            >
              <X className='size-3.5' />
            </button>
          </div>

          <div className='max-h-60 overflow-y-auto py-1'>
            {!projects?.length ? (
              <p className='px-4 py-3 text-xs text-ink-faint'>No projects found.</p>
            ) : (
              projects.map(project => {
                const isActive = activeProject?.ID === project.ID
                return (
                  <div
                    key={project.ID}
                    className='group/row flex items-center hover:bg-surface-high transition-colors'
                  >
                    <button
                      type='button'
                      onClick={() => {
                        if (project.ID !== activeProject?.ID) {
                          queryClient.removeQueries({ predicate: q => q.queryKey[0] !== 'projects' })
                          setActiveProject(project)
                          setOpen(false)
                          navigate('/')
                        } else {
                          setOpen(false)
                        }
                      }}
                      className={cn(
                        'cursor-pointer flex-1 flex items-center gap-2.5 px-3 py-2 text-left min-w-0',
                        isActive ? 'text-ink' : 'text-ink-dim',
                      )}
                    >
                      <span className='flex-1 min-w-0 text-xs font-medium truncate'>
                        {project.Name}
                      </span>
                      <span className='size-4 flex items-center justify-center shrink-0'>
                        {isActive && <Check className='size-3 text-primary' />}
                      </span>
                    </button>
                    {project.Key !== DEFAULT_PROJECT_KEY && (
                      <button
                        type='button'
                        onClick={e => {
                          e.stopPropagation()
                          setOpen(false)
                          setDeleteTarget(project)
                        }}
                        className='cursor-pointer shrink-0 mr-2 p-1 rounded opacity-0 group-hover/row:opacity-100 text-ink-faint hover:text-danger transition-all'
                        title='Delete project'
                      >
                        <Trash2 className='size-3' />
                      </button>
                    )}
                  </div>
                )
              })
            )}
          </div>

          <div className='border-t border-rim p-1'>
            <button
              type='button'
              onClick={() => { setOpen(false); setCreateOpen(true) }}
              className='cursor-pointer w-full flex items-center gap-2 rounded px-3 py-2 text-xs text-ink-faint hover:text-ink hover:bg-surface-high transition-colors'
            >
              <Plus className='size-3.5' />
              New project
            </button>
          </div>
        </div>,
        document.body,
      )}

      <CreateProjectModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => queryClient.invalidateQueries({ queryKey: ['projects'] })}
      />

      {deleteTarget && (
        <DeleteProjectModal
          project={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onDeleted={() => handleDeleted(deleteTarget)}
        />
      )}
    </>
  )
}
