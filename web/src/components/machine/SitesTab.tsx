import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Globe, FolderOpen, AlertCircle, Play, Square, RotateCw } from 'lucide-react'
import { RowMenu } from '@/components/ui/RowMenu'
import type { RowMenuItem } from '@/components/ui/RowMenu'
import { Pagination } from '@/components/ui/Pagination'
import { appPoolService } from '@/services/appPool.service'
import { cn } from '@/utils/cn'
import type { Binding, Site } from '@/types'

type StatusFilter = 'All' | 'Started' | 'Stopped'

const PER_PAGE = 25

const TH = 'px-4 py-2.5 text-left text-[0.625rem] font-semibold uppercase tracking-widest text-ink-faint'

const siteStatusCfg: Record<string, { dot: string; text: string }> = {
  Started:  { dot: 'bg-success', text: 'text-success' },
  Stopped:  { dot: 'bg-danger',  text: 'text-danger'  },
  Starting: { dot: 'bg-warning', text: 'text-warning' },
  Stopping: { dot: 'bg-warning', text: 'text-warning' },
}

function formatBinding(b: Binding) {
  const host = b.hostname || b.ip || '*'
  return `${b.protocol}://${host}:${b.port}`
}

function Skeleton() {
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-7 w-20 rounded-md bg-surface-high animate-pulse" />
        ))}
      </div>
      <div className="rounded-lg border border-rim overflow-hidden">
        <div className="border-b border-rim bg-surface-alt px-5 py-2.5 flex gap-6">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-2.5 w-24 rounded bg-surface-high animate-pulse" />
          ))}
        </div>
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="flex items-center gap-6 px-5 py-4 border-b border-rim last:border-0 animate-pulse">
            <div className="size-6 rounded border border-rim bg-surface-high shrink-0" />
            {Array.from({ length: 4 }).map((_, j) => (
              <div key={j} className="flex-1 h-2.5 rounded bg-surface-high" />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

const STATUS_FILTERS: StatusFilter[] = ['All', 'Started', 'Stopped']

export function SitesTab({
  projectKey,
  machineId,
}: {
  projectKey: string
  machineId: number
}) {
  const [acting, setActing] = useState<Record<number, string>>({})
  const [status, setStatus] = useState<StatusFilter>('Started')
  const [page, setPage] = useState(1)

  const apiStatus = status === 'All' ? undefined : status

  const { data, isLoading, isError } = useQuery({
    queryKey: ['sites', projectKey, machineId, apiStatus, page, PER_PAGE],
    enabled: !!projectKey && !!machineId,
    refetchInterval: 5_000,
    queryFn: async () => {
      const { data } = await appPoolService.listSites(projectKey, machineId, {
        ...(apiStatus ? { status: apiStatus } : {}),
        page,
        per_page: PER_PAGE,
      })
      return { sites: (data.sites ?? []) as Site[], total: (data.total ?? 0) as number }
    },
  })

  const paginated = data?.sites ?? []
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PER_PAGE))

  function handleStatusChange(s: StatusFilter) {
    setStatus(s)
    setPage(1)
  }

  async function runCmd(site: Site, cmd: 'start' | 'stop' | 'restart') {
    setActing((prev) => ({ ...prev, [site.ID]: cmd }))
    try {
      await appPoolService.siteCommand(projectKey, machineId, site.ID, cmd)
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Command failed. Please try again.'
      toast.error(msg, { description: `${site.Name} - ${cmd}` })
    } finally {
      setActing((prev) => {
        const n = { ...prev }
        delete n[site.ID]
        return n
      })
    }
  }

  if (isLoading) return <Skeleton />

  if (isError) {
    return (
      <div className="flex items-center gap-2.5 rounded-lg border border-danger/20 bg-danger/5 px-4 py-3 text-xs text-danger">
        <AlertCircle className="size-4 shrink-0" />
        Failed to load sites.
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* Filter bar */}
      <div className="flex items-center gap-1.5">
        {STATUS_FILTERS.map(s => (
          <button
            key={s}
            type="button"
            onClick={() => handleStatusChange(s)}
            className={cn(
              'px-3 py-1 rounded-md text-xs font-medium transition-colors cursor-pointer',
              status === s
                ? 'bg-primary/10 text-primary'
                : 'text-ink-faint hover:text-ink hover:bg-surface-high',
            )}
          >
            {s}
          </button>
        ))}
      </div>

      {/* Table */}
      {paginated.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 rounded-lg border border-rim bg-surface text-center">
          <Globe className="size-5 text-ink-faint mb-3" />
          <p className="text-xs text-ink-faint">
            {status === 'All' ? 'No sites found on this host.' : `No ${status.toLowerCase()} sites.`}
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-rim overflow-hidden">
          <div className="grid grid-cols-[2fr_1fr_2fr_2fr_1.5fr_auto] border-b border-rim bg-surface-alt">
            <div className={TH}>Site Name</div>
            <div className={TH}>Status</div>
            <div className={TH}>Bindings</div>
            <div className={TH}>Physical Path</div>
            <div className={TH}>App Pool</div>
            <div className='px-4 py-2.5' />
          </div>

          {(paginated as Site[]).map(site => {
            const statusCfg = siteStatusCfg[site.State] ?? { dot: 'bg-ink-faint', text: 'text-ink-faint' }
            const bindings = (site.Bindings ?? []).map(formatBinding)
            const busy = acting[site.ID]
            const started = site.State === 'Started'
            const menuItems: RowMenuItem[] = started
              ? [
                  { icon: Square, label: 'Stop', variant: 'danger', onClick: () => runCmd(site, 'stop') },
                  { icon: RotateCw, label: 'Restart', onClick: () => runCmd(site, 'restart') },
                ]
              : [{ icon: Play, label: 'Start', onClick: () => runCmd(site, 'start') }]
            return (
              <div
                key={site.ID}
                className="grid grid-cols-[2fr_1fr_2fr_2fr_1.5fr_auto] items-center border-b border-rim last:border-0 hover:bg-surface-alt transition-colors duration-100"
              >
                <div className="flex items-center gap-3 px-4 py-3.5">
                  <div className="flex size-6 shrink-0 items-center justify-center rounded border border-rim bg-surface-high">
                    <Globe className="size-3 text-ink-faint" />
                  </div>
                  <span className="font-mono text-xs text-ink truncate">{site.Name}</span>
                </div>

                <div className="px-4 py-3.5">
                  <div className="flex items-center gap-2">
                    <span className={cn('size-1.5 rounded-full shrink-0', statusCfg.dot)} />
                    <span className={cn('text-xs', statusCfg.text)}>{site.State}</span>
                  </div>
                </div>

                <div className="px-4 py-3.5 space-y-0.5">
                  {bindings.length ? bindings.map((b, i) => (
                    <p key={i} className="font-mono text-[0.6875rem] text-ink-dim">{b}</p>
                  )) : <span className="text-xs text-ink-faint">N/A</span>}
                </div>

                <div className="flex items-center gap-2 px-4 py-3.5">
                  <FolderOpen className="size-3 text-ink-faint shrink-0" />
                  <span className="font-mono text-[0.6875rem] text-ink-dim truncate">{site.PhysicalPath || 'N/A'}</span>
                </div>

                <div className="px-4 py-3.5">
                  <span className="text-xs text-ink-dim">{site.AppPoolName || 'N/A'}</span>
                </div>

                <div className="flex items-center justify-center px-2 py-3">
                  <RowMenu items={menuItems} disabled={!!busy} />
                </div>
              </div>
            )
          })}

        </div>
      )}

      <Pagination
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />
    </div>
  )
}
