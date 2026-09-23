import type { InputHTMLAttributes, SelectHTMLAttributes } from "react";
import { cn } from "../../lib/cn";
export function Input({
  className,
  ...p
}: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "h-10 rounded-[10px] border border-[var(--border)] bg-[var(--surface)] px-3 text-sm shadow-[var(--shadow-sm)] transition-[border-color,box-shadow,background-color] duration-180 hover:border-[var(--border-strong)] text-[var(--ink)] placeholder:text-[var(--muted)]",
        className,
      )}
      {...p}
    />
  );
}
export function Select({
  className,
  ...p
}: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        "h-10 rounded-[10px] border border-[var(--border)] bg-[var(--surface)] px-3 text-sm shadow-[var(--shadow-sm)] transition-[border-color,box-shadow,background-color] duration-180 hover:border-[var(--border-strong)] text-[var(--ink)]",
        className,
      )}
      {...p}
    />
  );
}
export function Segmented<T extends string>({
  value,
  onChange,
  options,
  label,
}: {
  value: T;
  onChange: (v: T) => void;
  options: Array<{ value: T; label: string }>;
  label: string;
}) {
  return (
    <div
      className="inline-flex rounded-[10px] border border-[var(--border)] bg-[var(--surface-subtle)] p-1"
      role="group"
      aria-label={label}
    >
      {options.map((o) => (
        <button
          key={o.value}
          onClick={() => onChange(o.value)}
          aria-pressed={value === o.value}
          className={cn(
            "h-7 rounded-[7px] px-3 text-xs font-semibold transition-[color,background-color,box-shadow] duration-180",
            value === o.value
              ? "bg-[var(--surface)] text-[var(--primary)] shadow-[var(--shadow-sm)]"
              : "muted",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
export function Toggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm">
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cn(
          "relative h-5 w-9 rounded-full transition-colors",
          checked ? "bg-[var(--primary)]" : "bg-slate-400",
        )}
      >
        <span
          className={cn(
            "absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform",
            checked ? "translate-x-0.5" : "-translate-x-4",
          )}
        />
      </button>
      {label}
    </label>
  );
}
