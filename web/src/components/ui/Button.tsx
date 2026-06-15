import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/utils/cn";

type Variant = "primary" | "ghost" | "outline" | "danger";
type Size = "sm" | "md" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
}

const variants: Record<Variant, string> = {
  // Near-white bg + near-black text - GitHub's high-contrast CTA
  primary: "bg-primary text-canvas border border-primary/20 hover:bg-primary-hover hover:border-primary/30",
  // Borderless text button - GitHub's ghost/link style
  ghost: "text-ink-dim hover:text-ink hover:bg-surface-high",
  // Bordered default button - GitHub's standard button
  outline: "bg-surface-high border border-rim text-ink-dim hover:text-ink hover:border-rim-strong",
  // Destructive - red text, bordered, fills red on hover
  danger: "text-danger border border-rim hover:bg-danger hover:text-white hover:border-danger",
};

const sizes: Record<Size, string> = {
  sm: "text-xs px-3 py-1.5 rounded-md gap-1.5",
  md: "text-sm px-3.5 py-1.5 rounded-md gap-2",
  lg: "text-sm px-4 py-2 rounded-md gap-2",
};

export function Button({
  variant = "primary",
  size = "md",
  loading,
  disabled,
  className,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      disabled={disabled || loading}
      className={cn(
        "inline-flex items-center justify-center font-medium transition-colors duration-100",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1 focus-visible:ring-offset-canvas",
        "disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer",
        variants[variant],
        sizes[size],
        className
      )}
      {...props}
    >
      {loading && (
        <span className='size-3.5 rounded-full border-2 border-current border-t-transparent animate-spin shrink-0' />
      )}
      {children}
    </button>
  );
}
