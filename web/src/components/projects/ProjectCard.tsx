import { Globe, Calendar } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { cn } from '@/utils/cn'
import type { Project, ProjectStatus } from '@/types'

interface ProjectCardProps {
  project: Project
  onClick: () => void
}

function statusVariant(status: ProjectStatus) {
  if (status === 'ACTIVE') return 'success'
  if (status === 'PAUSED') return 'warning'
  return 'neutral'
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function ProjectCard({ project, onClick }: ProjectCardProps) {
  const creatorEmail =
    project.CreatedBy?.['Email'] ?? project.CreatedBy?.['Name'] ?? 'N/A'

  return (
    <div
      onClick={onClick}
      className={cn(
        'group cursor-pointer rounded-lg border border-rim bg-surface p-5',
        'hover:border-rim-strong hover:bg-surface-alt transition-all duration-150',
      )}
    >
      {/* Top row: name + status */}
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold text-ink truncate group-hover:text-primary transition-colors">
            {project.Name}
          </h3>
          <code className="text-[11px] font-mono text-ink-faint">{project.Key}</code>
        </div>
        <Badge variant={statusVariant(project.Status)}>{project.Status}</Badge>
      </div>

      {/* Domain */}
      <div className="flex items-center gap-1.5 text-xs text-ink-dim mb-4">
        <Globe className="size-3 shrink-0 text-ink-faint" />
        <span className="truncate">{project.BusinessDomain || 'N/A'}</span>
      </div>

      {/* Footer: created by + date */}
      <div className="flex items-center justify-between text-[11px] text-ink-faint border-t border-rim pt-3">
        <span className="truncate max-w-[55%]" title={creatorEmail}>
          {creatorEmail}
        </span>
        <div className="flex items-center gap-1 shrink-0">
          <Calendar className="size-3" />
          <span>{formatDate(project.CreatedAt)}</span>
        </div>
      </div>
    </div>
  )
}
