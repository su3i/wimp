import { useNavigate } from 'react-router-dom'
import { NavLink } from 'react-router-dom'
import { LayoutDashboard, Boxes, Database, HardDrive, Bell, CircleAlert, Settings, LogOut, PanelLeftOpen, PanelLeftClose } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { cn } from '@/utils/cn'
import { useAuthStore } from '@/store/auth'
import { useUIStore } from '@/store/ui'
import { authService } from '@/services/auth.service'
import { machineService } from '@/services/machine.service'
import { useProjectStore } from '@/store/project'
import { useQuery } from '@tanstack/react-query'
import type { HostRole } from '@/types'
import { ProjectSwitcher } from './ProjectSwitcher'
import iconSrc from '@/assets/icon.svg'

interface NavItem {
  to: string
  icon: LucideIcon
  label: string
  end?: boolean
  // Shown only when some host in the project runs this workload, so a project without IIS
  // has no Applications entry leading to an empty page.
  role?: HostRole
}

// What the platform is for: the fleet, what runs on it, and what is broken.
const NAV_ITEMS: NavItem[] = [
  { to: '/', icon: LayoutDashboard, label: 'Overview', end: true },
  { to: '/applications', icon: Boxes, label: 'Applications', role: 'iis' },
  { to: '/sql-instances', icon: Database, label: 'SQL Server Instances', role: 'sql' },
  { to: '/hosts', icon: HardDrive, label: 'Hosts' },
  { to: '/incidents', icon: CircleAlert, label: 'Incidents' },
]

// Supporting destinations, pinned to the bottom. Neither is somewhere you go to do the
// job - the raw event feed is what incidents are assembled from, and settings is
// configuration - so neither should compete with the four above for the top of the list.
const SECONDARY_NAV_ITEMS: NavItem[] = [
  { to: '/activity', icon: Bell, label: 'Activity' },
  { to: '/settings', icon: Settings, label: 'Settings' },
]

const tooltip = cn(
  'absolute left-full ml-3 px-2.5 py-1.5 rounded-md z-50',
  'bg-surface border border-rim shadow-lg',
  'text-sm font-medium text-ink whitespace-nowrap pointer-events-none',
  'opacity-0 -translate-x-1.5 group-hover:opacity-100 group-hover:translate-x-0',
  'transition-all duration-150 ease-out',
)

function NavItemLink({ item, expanded }: { item: NavItem; expanded: boolean }) {
  const { to, icon: Icon, label, end } = item
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) => cn(
        'relative group cursor-pointer flex items-center h-8 w-full rounded-md transition-colors',
        expanded ? 'gap-2.5 px-3' : 'justify-center',
        isActive ? 'bg-primary/10 text-primary' : 'text-ink-dim hover:bg-surface-high hover:text-ink',
      )}
    >
      <Icon className='size-3.5 shrink-0' />
      {expanded
        ? <span className='text-xs font-medium truncate'>{label}</span>
        : <span className={tooltip}>{label}</span>
      }
    </NavLink>
  )
}

function NavItemSkeleton({ expanded }: { expanded: boolean }) {
  return (
    <div className={cn('flex h-8 w-full items-center rounded-md animate-pulse', expanded ? 'gap-2.5 px-3' : 'justify-center')} aria-hidden='true'>
      <span className='size-3.5 shrink-0 rounded bg-surface-high' />
      {expanded && <span className='h-2.5 w-24 rounded bg-surface-high' />}
    </div>
  )
}

export function Sidebar() {
  const { user, refreshToken, clearAuth } = useAuthStore()
  const navigate = useNavigate()

  const { sidebarExpanded: expanded, setSidebarExpanded } = useUIStore()
  const { activeProject } = useProjectStore()

  // Same query and cache key as the dashboard and host pages, so this costs no extra request.
  const { data: machines, isError: machinesError } = useQuery({
    queryKey: ['machines', activeProject?.Key],
    enabled: !!activeProject,
    staleTime: 60_000,
    queryFn: async () => (await machineService.list(activeProject!.Key, { per_page: 100 })).data.machines ?? [],
  })
  const roles = new Set((machines ?? []).flatMap(m => m.roles ?? []))
  const rolesLoading = !activeProject || (machines === undefined && !machinesError)

  function setExpanded(v: boolean) {
    localStorage.setItem('wimp_sidebar_expanded', String(v))
    setSidebarExpanded(v)
  }

  async function handleLogout() {
    if (refreshToken) {
      try { await authService.logout(refreshToken) } catch { /* best-effort */ }
    }
    clearAuth()
    navigate('/login', { replace: true })
  }

  return (
    <aside className={cn(
      'flex shrink-0 flex-col border-r border-rim bg-surface h-screen sticky top-0 transition-[width] duration-200 overflow-hidden',
      expanded ? 'w-52' : 'w-14',
    )} aria-busy={rolesLoading}>
      {rolesLoading && <span className='sr-only' role='status'>Loading</span>}
      {/* Brand - icon only, always centered */}
      <div className='flex items-center justify-center border-b border-rim py-3'>
        {rolesLoading
          ? <div className='size-[72px] rounded-full bg-surface-high animate-pulse' aria-hidden='true' />
          : <img src={iconSrc} alt='wimp' className='size-[72px]' />}
      </div>

      {/* Project switcher */}
      <div className='relative border-b border-rim px-2 py-2'>
        {/* Keep the switcher mounted so it can select the initial project. */}
        <div className={rolesLoading ? 'invisible' : undefined}>
          <ProjectSwitcher expanded={expanded} />
        </div>
        {rolesLoading && (
          <div className='absolute inset-x-2 top-2' aria-hidden='true'>
            <NavItemSkeleton expanded={expanded} />
          </div>
        )}
      </div>

      {/* Primary nav. flex-1 so it takes the slack, which is what pushes the secondary
          group down to sit against the account row. */}
      <nav className='flex-1 py-3 px-2 space-y-0.5 overflow-hidden'>
        {NAV_ITEMS.map((item) =>
          rolesLoading ? (
            <NavItemSkeleton key={item.to} expanded={expanded} />
          ) : !item.role || roles.has(item.role) ? (
            <NavItemLink key={item.to} item={item} expanded={expanded} />
          ) : null,
        )}
      </nav>

      {/* Secondary nav */}
      <div className='shrink-0 border-t border-rim px-2 py-2 space-y-0.5'>
        {SECONDARY_NAV_ITEMS.map((item) => (
          rolesLoading
            ? <NavItemSkeleton key={item.to} expanded={expanded} />
            : <NavItemLink key={item.to} item={item} expanded={expanded} />
        ))}
      </div>

      {/* Bottom */}
      <div className='shrink-0 border-t border-rim px-2 py-3'>
        {rolesLoading ? (
          <div className={cn('animate-pulse', expanded ? 'flex h-7 items-center justify-around' : 'space-y-0.5')} aria-hidden='true'>
            {[0, 1, 2].map((i) => (
              <div key={i} className={cn('flex items-center justify-center', expanded ? 'h-7 flex-1' : 'h-8 w-full')}>
                <div className='size-5 rounded-full bg-surface-high' />
              </div>
            ))}
          </div>
        ) : expanded ? (
          <>
            {/* Avatar + actions in one row */}
            <div className='flex items-center h-7'>
              <div className='flex-1 flex items-center justify-center'>
                <div className='flex size-6 items-center justify-center rounded-full bg-primary/20 text-[0.625rem] font-semibold leading-none text-primary uppercase select-none'>
                  {user?.username?.[0] ?? '?'}
                </div>
              </div>
              <div className='w-px h-4 bg-rim shrink-0' />
              <div className='flex-1 flex items-center justify-center'>
                <button
                  onClick={() => setExpanded(false)}
                  title='Collapse sidebar'
                  className='h-7 w-7 flex items-center justify-center rounded-md text-ink-faint hover:bg-surface-high hover:text-ink transition-colors cursor-pointer'
                >
                  <PanelLeftClose className='size-3.5' />
                </button>
              </div>
              <div className='w-px h-4 bg-rim shrink-0' />
              <div className='flex-1 flex items-center justify-center'>
                <button
                  onClick={handleLogout}
                  title='Sign out'
                  className='h-7 w-7 flex items-center justify-center rounded-md text-ink-faint hover:bg-surface-high hover:text-danger transition-colors cursor-pointer'
                >
                  <LogOut className='size-3.5' />
                </button>
              </div>
            </div>
          </>
        ) : (
          <div className='space-y-0.5'>
            {/* User avatar */}
            <div className='relative group flex items-center justify-center h-8 w-full'>
              <div className='flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/20 text-[0.625rem] font-semibold text-primary uppercase select-none'>
                {user?.username?.[0] ?? '?'}
              </div>
              <span className={cn(tooltip, 'text-xs font-normal')}>{user?.username ?? 'Unknown'}</span>
            </div>

            {/* Expand */}
            <button
              onClick={() => setExpanded(true)}
              className='relative group cursor-pointer flex h-8 w-full items-center justify-center rounded-md text-ink-faint hover:bg-surface-high hover:text-ink transition-colors'
            >
              <PanelLeftOpen className='size-4' />
              <span className={tooltip}>Expand sidebar</span>
            </button>

            {/* Logout */}
            <button
              onClick={handleLogout}
              className='relative group cursor-pointer flex h-8 w-full items-center justify-center rounded-md text-ink-faint hover:bg-surface-high hover:text-danger transition-colors'
            >
              <LogOut className='size-4' />
              <span className={cn(tooltip, 'text-danger group-hover:text-danger')}>Sign Out</span>
            </button>
          </div>
        )}
      </div>
    </aside>
  )
}
