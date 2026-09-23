import * as PopoverPrimitive from "@radix-ui/react-popover";
import type { ComponentProps } from "react";
import { cn } from "../../lib/cn";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;
const PopoverClose = PopoverPrimitive.Close;
const PopoverAnchor = PopoverPrimitive.Anchor;

function PopoverContent({ className, align = "start", sideOffset = 6, ...props }: ComponentProps<typeof PopoverPrimitive.Content>) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Content
        align={align}
        sideOffset={sideOffset}
        className={cn("z-50 rounded-lg border border-[var(--border)] bg-[var(--surface-raised)] p-3 text-[var(--ink)] shadow-[var(--shadow-overlay)] outline-none", className)}
        {...props}
      />
    </PopoverPrimitive.Portal>
  );
}

export { Popover, PopoverTrigger, PopoverContent, PopoverClose, PopoverAnchor };
