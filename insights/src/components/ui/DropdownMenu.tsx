import * as DropdownMenuPrimitive from "@radix-ui/react-dropdown-menu";
import { Check, ChevronRight } from "lucide-react";
import type { ComponentProps } from "react";
import { cn } from "../../lib/cn";

const DropdownMenu = DropdownMenuPrimitive.Root;
const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
const DropdownMenuGroup = DropdownMenuPrimitive.Group;
const DropdownMenuSub = DropdownMenuPrimitive.Sub;
const DropdownMenuRadioGroup = DropdownMenuPrimitive.RadioGroup;

function DropdownMenuContent({ className, sideOffset = 6, ...props }: ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return <DropdownMenuPrimitive.Portal><DropdownMenuPrimitive.Content sideOffset={sideOffset} className={cn("z-50 min-w-56 rounded-lg border border-[var(--border)] bg-[var(--surface-raised)] p-1.5 text-[var(--ink)] shadow-[var(--shadow-overlay)] outline-none", className)} {...props} /></DropdownMenuPrimitive.Portal>;
}
function DropdownMenuItem({ className, inset, ...props }: ComponentProps<typeof DropdownMenuPrimitive.Item> & { inset?: boolean }) {
  return <DropdownMenuPrimitive.Item className={cn("relative flex h-9 cursor-default select-none items-center gap-2 rounded-md px-2.5 text-sm outline-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-[var(--surface-subtle)]", inset && "pl-8", className)} {...props} />;
}
function DropdownMenuLabel({ className, inset, ...props }: ComponentProps<typeof DropdownMenuPrimitive.Label> & { inset?: boolean }) {
  return <DropdownMenuPrimitive.Label className={cn("px-2.5 py-1.5 text-xs font-semibold text-[var(--muted)]", inset && "pl-8", className)} {...props} />;
}
function DropdownMenuSeparator({ className, ...props }: ComponentProps<typeof DropdownMenuPrimitive.Separator>) {
  return <DropdownMenuPrimitive.Separator className={cn("-mx-1 my-1 h-px bg-[var(--border)]", className)} {...props} />;
}
function DropdownMenuCheckboxItem({ className, children, checked, ...props }: ComponentProps<typeof DropdownMenuPrimitive.CheckboxItem>) {
  return <DropdownMenuPrimitive.CheckboxItem checked={checked} className={cn("relative flex h-9 cursor-default select-none items-center rounded-md py-1.5 pl-8 pr-2 text-sm outline-none data-[highlighted]:bg-[var(--surface-subtle)]", className)} {...props}><span className="absolute left-2 grid size-4 place-items-center"><DropdownMenuPrimitive.ItemIndicator><Check /></DropdownMenuPrimitive.ItemIndicator></span>{children}</DropdownMenuPrimitive.CheckboxItem>;
}
function DropdownMenuSubTrigger({ className, inset, children, ...props }: ComponentProps<typeof DropdownMenuPrimitive.SubTrigger> & { inset?: boolean }) {
  return <DropdownMenuPrimitive.SubTrigger className={cn("flex h-9 cursor-default select-none items-center rounded-md px-2.5 text-sm outline-none data-[highlighted]:bg-[var(--surface-subtle)]", inset && "pl-8", className)} {...props}>{children}<ChevronRight className="ml-auto" /></DropdownMenuPrimitive.SubTrigger>;
}
function DropdownMenuSubContent({ className, ...props }: ComponentProps<typeof DropdownMenuPrimitive.SubContent>) {
  return <DropdownMenuPrimitive.SubContent className={cn("z-50 min-w-44 rounded-lg border border-[var(--border)] bg-[var(--surface-raised)] p-1.5 shadow-[var(--shadow-overlay)]", className)} {...props} />;
}
const DropdownMenuRadioItem = DropdownMenuPrimitive.RadioItem;

export { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuGroup, DropdownMenuSub, DropdownMenuSubTrigger, DropdownMenuSubContent, DropdownMenuCheckboxItem, DropdownMenuRadioGroup, DropdownMenuRadioItem };
