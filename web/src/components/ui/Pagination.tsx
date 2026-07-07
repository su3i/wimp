import { ChevronLeft, ChevronRight } from 'lucide-react'
import { cn } from '@/utils/cn'

interface PaginationProps {
  page: number
  totalPages: number
  total?: number
  pageSize?: number
  onPageChange: (page: number) => void
  className?: string
}

function pageWindow(page: number, totalPages: number): (number | '...')[] {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, i) => i + 1)

  const pages: (number | '...')[] = [1]

  if (page > 3) pages.push('...')

  const start = Math.max(2, page - 1)
  const end = Math.min(totalPages - 1, page + 1)
  for (let i = start; i <= end; i++) pages.push(i)

  if (page < totalPages - 2) pages.push('...')

  pages.push(totalPages)
  return pages
}

export function Pagination({ page, totalPages, total, pageSize, onPageChange, className }: PaginationProps) {
  if (totalPages <= 1) return null

  const from = total && pageSize ? (page - 1) * pageSize + 1 : null
  const to   = total && pageSize ? Math.min(page * pageSize, total) : null

  const pages = pageWindow(page, totalPages)

  const btnBase = 'flex items-center justify-center size-7 rounded-md border transition-colors cursor-pointer text-xs tabular-nums'

  return (
    <div className={cn('flex flex-col items-center gap-3 py-4 text-xs text-ink-faint', className)}>
      {/* Page controls */}
      <div className='flex items-center gap-1.5'>
        <button
          type='button'
          onClick={() => onPageChange(page - 1)}
          disabled={page === 1}
          className={cn(btnBase, 'border-rim bg-surface hover:bg-surface-high disabled:opacity-30 disabled:cursor-not-allowed')}
          aria-label='Previous page'
        >
          <ChevronLeft className='size-3.5' />
        </button>

        <div className='flex items-center gap-1'>
          {pages.map((p, i) =>
            p === '...' ? (
              <span key={`ellipsis-${i}`} className='flex items-center justify-center w-7 select-none tracking-widest'>
                ···
              </span>
            ) : (
              <button
                key={p}
                type='button'
                onClick={() => onPageChange(p as number)}
                className={cn(
                  btnBase,
                  p === page
                    ? 'border-primary/40 bg-primary/10 text-primary font-semibold'
                    : 'border-rim bg-surface hover:bg-surface-high text-ink-faint hover:text-ink',
                )}
              >
                {p}
              </button>
            )
          )}
        </div>

        <button
          type='button'
          onClick={() => onPageChange(page + 1)}
          disabled={page === totalPages}
          className={cn(btnBase, 'border-rim bg-surface hover:bg-surface-high disabled:opacity-30 disabled:cursor-not-allowed')}
          aria-label='Next page'
        >
          <ChevronRight className='size-3.5' />
        </button>
      </div>

      {/* Showing info below buttons */}
      {from && to && total && (
        <span className='tabular-nums text-[0.6875rem] text-ink-faint'>
          Showing <span className='text-ink-dim font-medium'>{from}–{to}</span> of <span className='text-ink-dim font-medium'>{total}</span>
        </span>
      )}
    </div>
  )
}
