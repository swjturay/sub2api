import * as SelectPrimitive from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { Children, isValidElement, useState, type ChangeEvent, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

function optionText(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (isValidElement<{ children?: ReactNode }>(node)) return optionText(node.props.children);
  if (Array.isArray(node)) return node.map(optionText).join("");
  return "";
}
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
  children,
  value,
  defaultValue,
  onChange,
  disabled,
  id,
  name,
  required,
  "aria-label": ariaLabel,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement>) {
  const options = Children.toArray(children).flatMap((child) => {
    if (!isValidElement<{ value?: string | number; disabled?: boolean; children?: ReactNode }>(child) || child.type !== "option") return [];
    return [{ value: String(child.props.value ?? optionText(child.props.children)), label: child.props.children, disabled: child.props.disabled }];
  });
  const emptySentinel = "__insights_empty_value__";
  const normalize = (next: string | number | readonly string[] | undefined) => String(Array.isArray(next) ? next[0] ?? "" : next ?? "") || emptySentinel;
  const [uncontrolledValue, setUncontrolledValue] = useState(normalize(defaultValue));
  const currentValue = value === undefined ? uncontrolledValue : normalize(value);
  const handleChange = (next: string) => {
    const actual = next === emptySentinel ? "" : next;
    if (value === undefined) setUncontrolledValue(next);
    onChange?.({ target: { value: actual }, currentTarget: { value: actual } } as ChangeEvent<HTMLSelectElement>);
  };
  return (
    <SelectPrimitive.Root value={currentValue} onValueChange={handleChange} disabled={disabled} name={name} required={required}>
      <SelectPrimitive.Trigger
        id={id}
        aria-label={ariaLabel}
        className={cn("inline-flex h-9 min-w-28 items-center justify-between gap-2 rounded-md border border-[var(--border)] bg-[var(--surface)] px-3 text-sm text-[var(--ink)] shadow-[var(--shadow-sm)] outline-none transition-colors hover:border-[var(--border-strong)] focus-visible:ring-2 focus-visible:ring-[var(--ring)] disabled:pointer-events-none disabled:opacity-50", className)}
        {...props as Record<string, unknown>}
      >
        <SelectPrimitive.Value />
        <SelectPrimitive.Icon><ChevronDown aria-hidden="true" /></SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content position="popper" sideOffset={5} className="z-50 max-h-80 min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-raised)] text-[var(--ink)] shadow-[var(--shadow-overlay)]">
          <SelectPrimitive.ScrollUpButton className="grid h-7 place-items-center"><ChevronUp /></SelectPrimitive.ScrollUpButton>
          <SelectPrimitive.Viewport className="p-1.5">
            {options.map((option) => (
              <SelectPrimitive.Item key={option.value || emptySentinel} value={option.value || emptySentinel} disabled={option.disabled} className="relative flex h-9 cursor-default select-none items-center rounded-md py-1.5 pl-8 pr-3 text-sm outline-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-[var(--surface-subtle)]">
                <span className="absolute left-2 grid size-4 place-items-center"><SelectPrimitive.ItemIndicator><Check /></SelectPrimitive.ItemIndicator></span>
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
          <SelectPrimitive.ScrollDownButton className="grid h-7 place-items-center"><ChevronDown /></SelectPrimitive.ScrollDownButton>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
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
      className="segmented-control inline-flex rounded-md border border-[var(--border)] bg-[var(--surface-subtle)] p-0.5"
      role="group"
      aria-label={label}
    >
      {options.map((o) => (
        <button
          type="button"
          key={o.value}
          onClick={() => onChange(o.value)}
          aria-pressed={value === o.value}
          className={cn(
            "h-7 rounded-[5px] px-3 text-xs font-semibold transition-colors",
            value === o.value
              ? "bg-[var(--surface)] text-[var(--primary)] shadow-[var(--shadow-sm)]"
              : "text-[var(--muted)] hover:text-[var(--ink)]",
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
    <label className="flex cursor-pointer items-center gap-2 text-sm text-[var(--ink)]">
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cn(
          "relative h-5 w-9 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:ring-offset-2",
          checked ? "bg-[var(--primary)]" : "bg-[var(--control-off)]",
        )}
      >
        <span
          className={cn(
            "absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform",
            checked && "translate-x-4",
          )}
        />
      </button>
      {label}
    </label>
  );
}
