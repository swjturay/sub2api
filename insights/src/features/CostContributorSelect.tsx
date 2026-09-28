import { useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { Button } from "../components/ui/Button";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "../components/ui/Command";
import { Popover, PopoverContent, PopoverTrigger } from "../components/ui/Popover";
import type { CostContributor } from "../lib/types";

export function CostContributorSelect({ value, onChange, contributors, current, disabled }: {
  value: string;
  onChange: (value: string) => void;
  contributors: CostContributor[];
  current?: CostContributor;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const selected = contributors.find((person) => String(person.id) === value) || (String(current?.id) === value ? current : undefined);
  const query = search.trim().toLocaleLowerCase();
  const matches = contributors.filter((person) => `${person.name} ${person.email} ${person.department}`.toLocaleLowerCase().includes(query));
  const updateOpen = (next: boolean) => { setOpen(next); setSearch(""); };
  const summary = selected ? `${selected.name} · ${selected.department}` : "请选择现有用户";
  return (
    <Popover open={open} onOpenChange={updateOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" role="combobox" aria-label="账号贡献人" aria-expanded={open} disabled={disabled} className="w-full min-w-0 justify-between font-medium" title={summary}>
          <span className="min-w-0 truncate">{summary}</span>
          <ChevronsUpDown className="text-[var(--muted)]" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] min-w-72 max-w-[calc(100vw-32px)] p-0" align="start" aria-label="选择贡献人">
        <Command label="搜索贡献人" shouldFilter={false} defaultValue={value}>
          <CommandInput aria-label="搜索贡献人" placeholder="搜索姓名、邮箱或部门" value={search} onValueChange={setSearch} />
          <CommandList>
            <CommandEmpty>没有匹配的用户</CommandEmpty>
            <CommandGroup>
              {matches.map((person) => (
                <CommandItem key={person.id} value={String(person.id)} onSelect={() => { onChange(String(person.id)); updateOpen(false); }}>
                  <span className="grid size-4 shrink-0 place-items-center">{String(person.id) === value && <Check aria-label="已选择" />}</span>
                  <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">
                    <span className="block font-medium">{person.name}</span>
                    <span className="block text-xs text-[var(--muted)]">{person.department} · {person.email}</span>
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
