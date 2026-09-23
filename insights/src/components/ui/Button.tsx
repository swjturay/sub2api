import type { ButtonHTMLAttributes } from "react";
import { cn } from "../../lib/cn";
export function Button({
  className,
  variant = "primary",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
}) {
  return (
    <button
      className={cn(
        "inline-flex h-9 items-center justify-center gap-2 rounded-[9px] px-3 text-sm font-semibold transition-[background-color,color,border-color,transform] duration-180 active:translate-y-px disabled:cursor-not-allowed disabled:opacity-50",
        variant === "primary" &&
          "bg-[var(--primary)] text-white hover:bg-[var(--primary-hover)]",
        variant === "secondary" &&
          "border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-sm)] hover:border-[var(--border-strong)] hover:bg-[var(--surface-subtle)]",
        variant === "ghost" && "hover:bg-[var(--surface-subtle)]",
        variant === "danger" && "bg-[var(--danger)] text-white",
        className,
      )}
      {...props}
    />
  );
}
