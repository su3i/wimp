import { Info } from "lucide-react";

// Small hover explainer for dashboard tiles. Dashboard metrics are mostly derived
// Prometheus expressions whose meaning isn't obvious from a label and a number, so the
// tile explains itself rather than sending the reader to the docs.
export function InfoTooltip({ text }: { text: string }) {
  return (
    <span className='relative inline-flex group/info'>
      <Info className='size-3 text-ink-faint hover:text-ink-dim transition-colors cursor-help' />
      <span className='pointer-events-none absolute top-full left-1/2 -translate-x-1/2 mt-1.5 w-max max-w-[190px] px-2 py-1.5 rounded border border-rim bg-surface-highest text-[0.5625rem] normal-case tracking-normal font-normal text-ink-dim leading-snug opacity-0 group-hover/info:opacity-100 transition-opacity z-20 shadow-md'>
        {text}
      </span>
    </span>
  );
}
