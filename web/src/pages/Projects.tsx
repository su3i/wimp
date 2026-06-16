import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Plus, Layers, AlertCircle, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { CreateProjectModal } from '@/components/projects/CreateProjectModal'
import { projectService } from '@/services/project.service'
import { cn } from '@/utils/cn'
import type { ProjectStatus } from '@/types'

// ── Status dot + label ───────────────────────────────────────────────────────

const statusConfig: Record<ProjectStatus, { dot: string; label: string; text: string }> = {
  ACTIVE:   { dot: 'bg-success', label: 'Active',   text: 'text-success' },
  PAUSED:   { dot: 'bg-warning', label: 'Paused',   text: 'text-warning' },
  ARCHIVED: { dot: 'bg-ink-faint', label: 'Archived', text: 'text-ink-faint' },
}

function StatusCell({ status }: { status: ProjectStatus }) {
  const cfg = statusConfig[status]
  return (
    <div className="flex items-center gap-2">
      <span className={cn('size-1.5 rounded-full shrink-0', cfg.dot)} />
      <span className={cn('text-xs font-medium', cfg.text)}>{cfg.label}</span>
    </div>
  )
}

// ── Table skeleton ───────────────────────────────────────────────────────────

function TableSkeleton() {
  return (
    <div className="rounded-lg border border-rim overflow-hidden">
      <div className="border-b border-rim bg-surface-alt px-5 py-2.5 flex gap-8">
        {['w-24', 'w-32', 'w-20', 'w-16'].map((w, i) => (
          <div key={i} className={cn('h-2.5 rounded bg-surface-high animate-pulse', w)} />
        ))}
      </div>
      {Array.from({ length: 5 }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-8 px-5 py-4 border-b border-rim last:border-0 animate-pulse"
        >
          <div className="flex-[2] space-y-1.5">
            <div className="h-3 w-36 rounded bg-surface-high" />
            <div className="h-2 w-20 rounded bg-surface-high" />
          </div>
          <div className="flex-[2] h-2.5 w-28 rounded bg-surface-high" />
          <div className="flex-1 h-2.5 w-14 rounded bg-surface-high" />
          <div className="flex-1 space-y-1.5">
            <div className="h-2 w-20 rounded bg-surface-high" />
            <div className="h-2 w-28 rounded bg-surface-high" />
          </div>
        </div>
      ))}
    </div>
  )
}

// ── Empty state ──────────────────────────────────────────────────────────────

function EmptyState({ onNew }: { onNew: () => void }) {
  return (
    <div className="rounded-lg border border-rim bg-surface">
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <div className="mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface-alt">
          <Layers className="size-4 text-ink-faint" />
        </div>
        <p className="text-sm font-semibold text-ink">No projects yet</p>
        <p className="mt-1 max-w-xs text-xs text-ink-faint">
          Create your first project to start managing Windows servers and IIS workloads.
        </p>
        <Button className="mt-5" onClick={onNew} size="sm">
          <Plus className="size-3.5" />
          New project
        </Button>
      </div>
    </div>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

const TH = 'px-5 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'

export function Projects() {
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)

  const { data: projects, isLoading, isError, refetch } = useQuery({
    queryKey: ['projects'],
    queryFn: async () => {
      const { data } = await projectService.list()
      return data.projects ?? []
    },
  })

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-base font-semibold text-ink">Projects</h1>
          {projects !== undefined && (
            <p className="mt-0.5 text-xs text-ink-faint">
              {projects.length} {projects.length === 1 ? 'project' : 'projects'}
            </p>
          )}
        </div>
        <Button onClick={() => setCreateOpen(true)} size="sm">
          <Plus className="size-3.5" />
          New project
        </Button>
      </div>

      {/* Content */}
      {isLoading ? (
        <TableSkeleton />
      ) : isError ? (
        <div className="flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger">
          <AlertCircle className="size-4 shrink-0" />
          Failed to load projects. Check your connection and try again.
        </div>
      ) : !projects?.length ? (
        <EmptyState onNew={() => setCreateOpen(true)} />
      ) : (
        <div className="rounded-lg border border-rim overflow-hidden">
          {/* Table header */}
          <div className="grid grid-cols-[2fr_2fr_1fr_1fr_auto] border-b border-rim bg-surface-alt">
            <div className={TH}>Name</div>
            <div className={TH}>Domain</div>
            <div className={TH}>Status</div>
            <div className={TH}>Added</div>
            <div className={cn(TH, 'w-10')} />
          </div>

          {/* Rows */}
          {projects.map(project => {
            const creator =
              project.CreatedBy?.['Email'] ?? project.CreatedBy?.['Name'] ?? 'N/A'
            return (
              <div
                key={project.ID}
                onClick={() => navigate(`/projects/${project.Key}`)}
                className={cn(
                  'grid grid-cols-[2fr_2fr_1fr_1fr_auto] items-center',
                  'cursor-pointer border-b border-rim last:border-0',
                  'hover:bg-surface-alt transition-colors duration-100 group',
                )}
              >
                {/* Name + key */}
                <div className="px-5 py-3.5">
                  <p className="text-sm font-medium text-ink group-hover:text-primary transition-colors truncate">
                    {project.Name}
                  </p>
                  <code className="text-[0.625rem] font-mono text-ink-faint">{project.Key}</code>
                </div>

                {/* Domain */}
                <div className="px-5 py-3.5">
                  <span className="text-xs text-ink-dim truncate block">{project.BusinessDomain || 'N/A'}</span>
                </div>

                {/* Status */}
                <div className="px-5 py-3.5">
                  <StatusCell status={project.Status} />
                </div>

                {/* Date + creator */}
                <div className="px-5 py-3.5">
                  <p className="text-xs text-ink-dim">{formatDate(project.CreatedAt)}</p>
                  <p className="text-[0.625rem] text-ink-faint truncate max-w-[10rem]" title={creator}>
                    {creator}
                  </p>
                </div>

                {/* Chevron */}
                <div className="px-3 py-3.5 flex items-center justify-center w-10">
                  <ChevronRight className="size-3.5 text-ink-faint group-hover:text-ink-dim transition-colors" />
                </div>
              </div>
            )
          })}
        </div>
      )}

      <CreateProjectModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => void refetch()}
      />
    </div>
  )
}
