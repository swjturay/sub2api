import { Command as CommandPrimitive } from "cmdk";
import { Search } from "lucide-react";
import type { ComponentProps } from "react";
import { cn } from "../../lib/cn";

function Command({ className, ...props }: ComponentProps<typeof CommandPrimitive>) {
  return <CommandPrimitive className={cn("flex min-h-0 flex-col overflow-hidden", className)} {...props} />;
}
function CommandInput({ className, ...props }: ComponentProps<typeof CommandPrimitive.Input>) {
  return <div className="flex h-10 items-center gap-2 border-b border-[var(--border)] px-3"><Search className="size-4 text-[var(--muted)]" aria-hidden="true" /><CommandPrimitive.Input className={cn("h-full min-w-0 flex-1 bg-transparent text-sm text-[var(--ink)] outline-none placeholder:text-[var(--muted)]", className)} {...props} /></div>;
}
function CommandList({ className, ...props }: ComponentProps<typeof CommandPrimitive.List>) {
  return <CommandPrimitive.List className={cn("max-h-64 overflow-y-auto overflow-x-hidden p-1.5", className)} {...props} />;
}
function CommandEmpty({ className, ...props }: ComponentProps<typeof CommandPrimitive.Empty>) {
  return <CommandPrimitive.Empty className={cn("px-3 py-8 text-center text-sm text-[var(--muted)]", className)} {...props} />;
}
function CommandGroup({ className, ...props }: ComponentProps<typeof CommandPrimitive.Group>) {
  return <CommandPrimitive.Group className={cn("text-[var(--ink)] [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:text-[var(--muted)]", className)} {...props} />;
}
function CommandItem({ className, ...props }: ComponentProps<typeof CommandPrimitive.Item>) {
  return <CommandPrimitive.Item className={cn("relative flex min-h-9 cursor-default select-none items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 data-[selected=true]:bg-[var(--surface-subtle)]", className)} {...props} />;
}

export { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem };
