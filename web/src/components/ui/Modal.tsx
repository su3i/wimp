import { useEffect, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { cn } from '@/utils/cn'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  description?: string
  children: ReactNode
  className?: string
}

export function Modal({ open, onClose, title, description, children, className }: ModalProps) {
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div className='fixed inset-0 z-[9999] flex items-center justify-center p-4'>
      <div className='absolute inset-0 bg-black/90' onClick={onClose} />
      <div
        className={cn(
          'relative z-10 w-full max-w-[500px]',
          'rounded-xl border border-rim-strong bg-canvas',
          'shadow-[0_16px_48px_rgba(0,0,0,0.9)]',
          className,
        )}
      >
        {(title || description) && (
          <div className='flex items-start justify-between border-b border-rim px-5 py-4'>
            <div>
              {title && <h2 className='text-sm font-semibold text-ink'>{title}</h2>}
              {description && <p className='mt-0.5 text-xs text-ink-faint'>{description}</p>}
            </div>
            <button
              onClick={onClose}
              className='cursor-pointer -mt-0.5 ml-4 shrink-0 rounded p-0.5 text-ink-faint hover:text-ink hover:bg-surface-high transition-colors'
            >
              <X className='size-4' />
            </button>
          </div>
        )}
        <div className='px-5 py-4'>{children}</div>
      </div>
    </div>,
    document.body,
  )
}
