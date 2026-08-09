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

// Title is built server-side as "<HOST> // <event>" (see AlertTitle in
// internal/application/notification/service.go) - split it back into a host chip and
// event text rather than rendering the raw "X // Y" string.
export function splitTitle(title: string | null | undefined): { host: string; event: string } {
  const idx = (title ?? '').indexOf(' // ')
  if (idx === -1) return { host: '', event: title ?? '' }
  return { host: title!.slice(0, idx), event: title!.slice(idx + 4) }
}
