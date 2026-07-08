import { Monitor } from 'lucide-react'

export function MobileBlock() {
  return (
    <div className="fixed inset-0 z-[9999] flex flex-col items-center justify-center gap-5 bg-canvas p-8 lg:hidden">
      <div className="flex size-14 items-center justify-center rounded-xl border border-rim bg-surface">
        <Monitor className="size-6 text-ink-dim" />
      </div>
      <div className="text-center">
        <p className="text-sm font-semibold text-ink">Desktop only</p>
        <p className="mt-1.5 text-xs text-ink-faint leading-relaxed max-w-[260px]">
          WIMP is designed for desktop use only. Please open it on a larger screen.
        </p>
      </div>
    </div>
  )
}
