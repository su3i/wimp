import { Bell, Layers, Monitor, Server, Activity, Gauge, type LucideIcon } from 'lucide-react'

// Single source of truth for how notification Level/Category values map to visual
// treatment - was previously duplicated (and drifting) between Alerts.tsx and
// Dashboard.tsx.

export const LEVEL_CFG: Record<string, { label: string; cls: string }> = {
  sev: {
    label: 'sev+',
    cls: 'bg-danger/20 text-danger border border-danger/40 font-bold',
  },
  critical: {
    label: 'crit',
    cls: 'bg-danger/10 text-danger border border-danger/20',
  },
  warning: {
    label: 'warn',
    cls: 'bg-[#d29922]/10 text-[#d29922] border border-[#d29922]/20',
  },
  info: {
    label: 'info',
    cls: 'bg-surface-high text-ink-faint border border-rim',
  },
}

const FALLBACK_LEVEL_CFG = { label: 'info', cls: 'bg-surface-high text-ink-faint border border-rim' }

export function levelConfig(level: string | null | undefined) {
  if (!level) return FALLBACK_LEVEL_CFG
  return LEVEL_CFG[level] ?? { label: level, cls: FALLBACK_LEVEL_CFG.cls }
}

export function categoryIcon(cat: string | null | undefined): LucideIcon {
  if (cat === 'machine') return Monitor
  if (cat === 'apppool' || cat === 'app_pool') return Layers
  if (cat === 'iis') return Server
  if (cat === 'sidecar') return Activity
  if (cat === 'metrics') return Gauge
  return Bell
}

export function categoryLabel(cat: string | null | undefined): string {
  if (cat === 'machine') return 'Machine'
  if (cat === 'apppool' || cat === 'app_pool') return 'App Pool'
  if (cat === 'iis') return 'IIS'
  if (cat === 'service') return 'Service'
  if (cat === 'sidecar') return 'Sidecar'
  if (cat === 'metrics') return 'Metrics'
  return cat ?? 'System'
}
