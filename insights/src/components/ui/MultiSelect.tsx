import { Check, ChevronsUpDown, X } from "lucide-react";
import { cn } from "../../lib/cn";
import { Button } from "./Button";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "./Command";
import { Popover, PopoverContent, PopoverTrigger } from "./Popover";

export interface MultiSelectOption {
  value: string;
  label: string;
  keywords?: string;
  disabled?: boolean;
}

export function MultiSelect({
  label,
  value,
  onChange,
  options,
  allLabel = "All",
  selectedLabel = "selected",
  searchPlaceholder = "搜索",
  emptyText = "没有匹配项",
  className,
}: {
  label: string;
  value: string[];
  onChange: (value: string[]) => void;
  options: MultiSelectOption[];
  allLabel?: string;
  selectedLabel?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  className?: string;
}) {
  const selected = new Set(value);
  const firstSelected = options.find((option) => option.value === value[0]);
  const firstLabel = firstSelected?.label || `${value.length} ${selectedLabel}`;
  const summary = value.length === 0 ? allLabel : `${firstLabel}${value.length > 1 ? ` +${value.length - 1}` : ""}`;
  const toggle = (option: MultiSelectOption) => {
    if (option.disabled) return;
    onChange(selected.has(option.value) ? value.filter((item) => item !== option.value) : [...value, option.value]);
  };
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="secondary"
          role="combobox"
          aria-label={label}
          className={cn("filter-trigger w-60 max-w-72 justify-between font-medium", className)}
          title={summary}
        >
          {value.length === 0 ? <span className="truncate">{allLabel}</span> : <span className="flex min-w-0 flex-1 items-center gap-1.5"><span className="min-w-0 flex-1 truncate">{firstLabel}</span>{value.length > 1 && <span className="shrink-0 rounded bg-[var(--primary-soft)] px-1.5 py-0.5 text-xs text-[var(--primary)]">+{value.length - 1}</span>}</span>}
          <ChevronsUpDown className="text-[var(--muted)]" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0" align="start">
        <Command label={`${label}搜索`}>
          <CommandInput aria-label={`${label}搜索`} placeholder={searchPlaceholder} />
          <CommandList>
            <CommandEmpty>{emptyText}</CommandEmpty>
            <CommandGroup>
              {options.map((option) => (
                <CommandItem
                  key={option.value}
                  value={`${option.label} ${option.keywords || ""}`}
                  disabled={option.disabled}
                  onSelect={() => toggle(option)}
                >
                  <span
                    role="checkbox"
                    aria-checked={selected.has(option.value)}
                    aria-label={`${option.label} ${selected.has(option.value) ? "已选择" : "未选择"}`}
                    className={cn("grid size-4 shrink-0 place-items-center rounded border", selected.has(option.value) ? "border-[var(--primary)] bg-[var(--primary)] text-[var(--primary-foreground)]" : "border-[var(--border-strong)]")}
                  >
                    {selected.has(option.value) && <Check aria-hidden="true" />}
                  </span>
                  <span className="min-w-0 flex-1 break-words [overflow-wrap:anywhere]" title={option.label}>{option.label}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
          {value.length > 0 && (
            <div className="border-t border-[var(--border)] p-1.5">
              <Button type="button" variant="ghost" className="w-full justify-start" onClick={() => onChange([])}>
                <X aria-hidden="true" /> 清空选择
              </Button>
            </div>
          )}
        </Command>
      </PopoverContent>
    </Popover>
  );
}
