import { Hint } from '@/components/ui/Hint'
import { InfoTooltip } from '@/components/ui/InfoTooltip'
import { cn } from '@/utils/cn'
import { instanceStatus } from '@/utils/sql'
import type { SqlInstanceWithMachine, SqlState } from '@/types'

const stateTone: Record<SqlState, { dot: string; text: string }> = {
  Running: { dot: 'bg-success', text: 'text-success' },
  Stopped: { dot: 'bg-danger', text: 'text-danger' },
  Failed: { dot: 'bg-danger', text: 'text-danger' },
  Starting: { dot: 'bg-warning', text: 'text-warning' },
  Stopping: { dot: 'bg-warning', text: 'text-warning' },
  Unknown: { dot: 'bg-ink-faint', text: 'text-ink-faint' },
}

export function SqlStateBadge({ state, className }: { state: SqlState; className?: string }) {
  const tone = stateTone[state] ?? stateTone.Unknown
  return (
    <span className={cn('inline-flex items-center gap-2 text-xs font-medium', tone.text, className)}>
      <span className={cn('size-1.5 rounded-full shrink-0', tone.dot)} />
      {state}
    </span>
  )
}

const statusTone = {
  success: { dot: 'bg-success', text: 'text-success' },
  warning: { dot: 'bg-warning', text: 'text-warning' },
  danger: { dot: 'bg-danger', text: 'text-danger' },
  muted: { dot: 'bg-ink-faint', text: 'text-ink-faint' },
}

// The single status shown for an instance: engine and SQL Agent together.
export function SqlStatusBadge({ instance }: { instance: SqlInstanceWithMachine }) {
  const s = instanceStatus(instance, instance.machine.Status === 'online')
  const tone = statusTone[s.tone]
  return (
    <span className={cn('inline-flex items-center gap-2 text-xs font-medium whitespace-nowrap', tone.text)}>
      <span className={cn('size-1.5 rounded-full shrink-0', tone.dot)} />
      {s.label}
      {s.detail && <InfoTooltip text={s.detail} />}
    </span>
  )
}

// Shown where SQL read access is missing. One line; the agent's exact error is behind the info
// icon, since it names what is missing (a failed login, or one permission).
export function SqlCredentialsHint({ error }: { error: string }) {
  return (
    <Hint>
      <p className='flex items-center gap-1.5'>
        <span className='min-w-0 truncate'>
          Grant <span className='font-mono text-ink'>NT AUTHORITY\SYSTEM</span> VIEW SERVER STATE, VIEW ANY DEFINITION
          and db_datareader in msdb to see uptime, waits, backups, jobs and live queries.
        </span>
        <InfoTooltip text={error} />
      </p>
    </Hint>
  )
}
