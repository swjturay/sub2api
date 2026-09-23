import type { ButtonHTMLAttributes } from "react";
import { cva } from "class-variance-authority";
import { cn } from "../../lib/cn";

const buttonVariants = cva(
  "inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-md px-3 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--surface)] disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        primary: "bg-[var(--primary)] text-[var(--primary-foreground)] shadow-[var(--shadow-sm)] hover:bg-[var(--primary-hover)]",
        secondary: "border border-[var(--border)] bg-[var(--surface)] text-[var(--ink)] shadow-[var(--shadow-sm)] hover:border-[var(--border-strong)] hover:bg-[var(--surface-subtle)]",
        ghost: "text-[var(--muted)] hover:bg-[var(--surface-subtle)] hover:text-[var(--ink)]",
        danger: "bg-[var(--danger)] text-[var(--danger-foreground)] shadow-[var(--shadow-sm)] hover:bg-[var(--danger-hover)]",
      },
    },
    defaultVariants: { variant: "primary" },
  },
);
export function Button({
  className,
  variant = "primary",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
}) {
  return (
    <button
      className={cn(buttonVariants({ variant }), className)}
      {...props}
    />
  );
}
