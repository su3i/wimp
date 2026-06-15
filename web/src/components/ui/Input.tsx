import { type InputHTMLAttributes, type ReactNode } from 'react'
import { cn } from '@/utils/cn'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  hint?: string
  leftIcon?: ReactNode
  rightSlot?: ReactNode
}

export function Input({
  label,
  error,
  hint,
  leftIcon,
  rightSlot,
  className,
  id,
  ...props
}: InputProps) {
  const inputId = id ?? label?.toLowerCase().replace(/\s+/g, '-')

  return (
    <div className="space-y-1.5">
      {label && (
        <label htmlFor={inputId} className="block text-xs font-medium text-ink-dim">
          {label}
        </label>
      )}
      <div className="relative flex items-center">
        {leftIcon && (
          <span className="absolute left-3 text-ink-faint pointer-events-none">{leftIcon}</span>
        )}
        <input
          id={inputId}
          className={cn(
            'w-full rounded-md border bg-canvas px-3 py-1.5 text-sm text-ink',
            'placeholder:text-ink-faint',
            'transition-colors duration-100 focus:outline-none',
            error
              ? 'border-danger focus:border-danger'
              : 'border-rim focus:border-rim-strong',
            leftIcon  ? 'pl-9'  : null,
            rightSlot ? 'pr-10' : null,
            className,
          )}
          {...props}
        />
        {rightSlot && (
          <span className="absolute right-3 flex items-center">{rightSlot}</span>
        )}
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      {hint && !error && <p className="text-xs text-ink-faint">{hint}</p>}
    </div>
  )
}
