import { NavLink, useNavigate } from 'react-router-dom'
import { LayoutDashboard, Boxes, HardDrive, Bell, LogOut } from 'lucide-react'
import { cn } from '@/utils/cn'
import { useAuthStore } from '@/store/auth'
import { authService } from '@/services/auth.service'
import { ProjectSwitcher } from './ProjectSwitcher'
import iconSrc from '@/assets/icon.svg'

const NAV_ITEMS = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard', end: true },
  { to: '/applications', icon: Boxes, label: 'Applications' },
  { to: '/machines', icon: HardDrive, label: 'Hosts' },
  { to: '/alerts', icon: Bell, label: 'Alerts' },
]

const hoverLabel = cn(
  'absolute left-full ml-3 px-2.5 py-1.5 rounded-md z-50',
  'bg-surface border border-rim shadow-lg',
  'text-sm font-medium text-ink whitespace-nowrap pointer-events-none',
  'opacity-0 -translate-x-1.5 group-hover:opacity-100 group-hover:translate-x-0',
  'transition-all duration-150 ease-out',
)

export function Sidebar() {
  const { user, refreshToken, clearAuth } = useAuthStore()
  const navigate = useNavigate()

  async function handleLogout() {
    if (refreshToken) {
      try { await authService.logout(refreshToken) } catch { /* best-effort */ }
    }
    clearAuth()
    navigate('/login', { replace: true })
  }

  return (
    <aside className="flex shrink-0 flex-col w-14 border-r border-rim bg-surface h-screen sticky top-0">
      {/* Brand */}
      <div className="flex h-16 items-center justify-center border-b border-rim shrink-0">
        <img src={iconSrc} alt="wimp" className="size-8 rounded-lg" />
      </div>

      {/* Project switcher */}
      <div className="border-b border-rim px-2 py-2">
        <ProjectSwitcher />
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5">
        {NAV_ITEMS.map(({ to, icon: Icon, label, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              cn(
                'relative group cursor-pointer flex items-center justify-center h-8 w-full rounded-md transition-colors',
                isActive
                  ? 'bg-primary/10 text-primary'
                  : 'text-ink-dim hover:bg-surface-high hover:text-ink',
              )
            }
          >
            <Icon className="size-4 shrink-0" />
            <span className={hoverLabel}>{label}</span>
          </NavLink>
        ))}
      </nav>

      {/* Bottom */}
      <div className="shrink-0 border-t border-rim px-2 py-3 space-y-0.5">
        {/* User avatar */}
        <div className="relative group flex items-center justify-center h-8 w-full">
          <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/20 text-[10px] font-semibold text-primary uppercase select-none">
            {user?.email?.[0] ?? '?'}
          </div>
          <span className={cn(hoverLabel, 'text-xs font-normal')}>
            {user?.email ?? 'Unknown'}
          </span>
        </div>

        {/* Logout */}
        <button
          onClick={handleLogout}
          className="relative group cursor-pointer flex h-8 w-full items-center justify-center rounded-md text-ink-faint hover:bg-surface-high hover:text-danger transition-colors"
        >
          <LogOut className="size-4" />
          <span className={cn(hoverLabel, 'text-danger group-hover:text-danger')}>Sign Out</span>
        </button>
      </div>
    </aside>
  )
}
