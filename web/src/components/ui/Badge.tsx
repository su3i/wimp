import type { HTMLAttributes } from "react";
import { cn } from "@/utils/cn";

type BadgeVariant = "default" | "success" | "warning" | "danger" | "neutral";

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
}

const variants: Record<BadgeVariant, string> = {
  // GitHub info blue - hardcoded since primary is now near-white (#f0f6fc)
  default: "bg-[#1f6feb]/20 text-[#2f81f7] ring-[#1f6feb]/30",
  success: "bg-success/10 text-success ring-success/20",
  warning: "bg-warning/10 text-warning ring-warning/20",
  danger: "bg-danger/10 text-danger ring-danger/20",
  neutral: "bg-surface-high text-ink-faint ring-rim",
};

export function Badge({ variant = "default", className, children, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-[0.625rem] font-medium uppercase tracking-wide ring-1 ring-inset",
        variants[variant],
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
}
