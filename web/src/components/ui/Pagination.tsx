import { ChevronLeft, ChevronRight } from 'lucide-react'
import { cn } from '@/utils/cn'

interface PaginationProps {
  page: number
  totalPages: number
  total?: number
  onPageChange: (page: number) => void
  itemLabel?: string
  className?: string
}

export function Pagination({ page, totalPages, total, onPageChange, itemLabel = 'item', className }: PaginationProps) {
  if (totalPages <= 1) return null

  return (
    <div className={cn('flex items-center justify-between text-xs text-ink-faint', className)}>
      <span />
      <div className='flex items-center gap-1'>
        <button
          type='button'
          onClick={() => onPageChange(page - 1)}
          disabled={page === 1}
          className='flex items-center justify-center size-7 rounded-md border border-rim bg-surface hover:bg-surface-high disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors'
        >
          <ChevronLeft className='size-3.5' />
        </button>
        <span className='px-2.5 tabular-nums'>{page} / {totalPages}</span>
        <button
          type='button'
          onClick={() => onPageChange(page + 1)}
          disabled={page === totalPages}
          className='flex items-center justify-center size-7 rounded-md border border-rim bg-surface hover:bg-surface-high disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors'
        >
          <ChevronRight className='size-3.5' />
        </button>
      </div>
    </div>
  )
}
