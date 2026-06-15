import { useState, useRef, useEffect } from 'react'
import { createPortal } from 'react-dom'
import { MoreVertical } from 'lucide-react'
import { cn } from '@/utils/cn'
import type { LucideIcon } from 'lucide-react'

export type RowMenuItem =
  | { type?: undefined; icon: LucideIcon; label: string; onClick: () => void; disabled?: boolean; variant?: 'danger' }
  | { type: 'separator' }

interface RowMenuProps {
  items: RowMenuItem[]
  disabled?: boolean
}

export function RowMenu({ items, disabled }: RowMenuProps) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState({ top: 0, left: 0 })
  const btnRef = useRef<HTMLButtonElement>(null)

  function toggle(e: React.MouseEvent) {
    e.stopPropagation()
    if (!open && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect()
      setPos({ top: rect.bottom + 4, left: rect.right })
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

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        onClick={toggle}
        disabled={disabled}
        className="flex items-center justify-center size-7 rounded hover:bg-surface-high transition-colors cursor-pointer text-ink-faint hover:text-ink disabled:opacity-40 disabled:cursor-not-allowed"
      >
        <MoreVertical className="size-3.5" />
      </button>

      {open && createPortal(
        <div
          onMouseDown={e => e.stopPropagation()}
          style={{ position: 'fixed', top: pos.top, left: pos.left, transform: 'translateX(-100%)' }}
          className="z-[9999] min-w-[160px] rounded-md border border-rim bg-surface-highest shadow-[0_8px_24px_rgba(1,4,9,0.7)] py-1"
        >
          {items.map((item, i) =>
            item.type === 'separator' ? (
              <div key={i} className="my-1 border-t border-rim" />
            ) : (
              <button
                key={i}
                type="button"
                disabled={item.disabled}
                onClick={(e) => {
                  e.stopPropagation()
                  setOpen(false)
                  item.onClick()
                }}
                className={cn(
                  'w-full flex items-center gap-2.5 px-3 py-2 text-xs transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed',
                  item.variant === 'danger'
                    ? 'text-danger hover:bg-danger/5'
                    : 'text-ink hover:bg-surface-high',
                )}
              >
                <item.icon className="size-3" />
                {item.label}
              </button>
            )
          )}
        </div>,
        document.body,
      )}
    </>
  )
}
