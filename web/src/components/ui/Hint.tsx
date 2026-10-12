import type { ReactNode } from 'react'
import { Info, X } from 'lucide-react'

// A quiet, inline nudge toward something worth setting up. Dismissible when onDismiss is given.
export function Hint({ children, onDismiss }: { children: ReactNode; onDismiss?: () => void }) {
  return (
    <div className='flex items-start gap-2.5 rounded-lg border border-rim bg-surface px-4 py-3'>
      <Info className='size-3.5 text-ink-dim shrink-0 mt-0.5' />
      <div className='text-xs text-ink-dim leading-relaxed flex-1'>{children}</div>
      {onDismiss && (
        <button
          type='button'
          onClick={onDismiss}
          className='cursor-pointer shrink-0 flex items-center justify-center size-6 rounded-md border-[1.5px] border-rim text-ink-faint hover:text-ink hover:border-ink-faint transition-colors'
        >
          <X className='size-3' />
        </button>
      )}
    </div>
  )
}
