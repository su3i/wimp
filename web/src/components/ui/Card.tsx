import type { HTMLAttributes } from "react";
import { cn } from "@/utils/cn";

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: "sm" | "md" | "lg";
}

const paddings = { sm: "p-4", md: "p-5", lg: "p-6" };

export function Card({ padding = "md", className, children, ...props }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-lg border border-rim bg-surface",
        paddings[padding],
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}
