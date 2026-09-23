import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ComponentProps } from "react";
import { cn } from "../../lib/cn";

const TooltipProvider = TooltipPrimitive.Provider;
const Tooltip = TooltipPrimitive.Root;
const TooltipTrigger = TooltipPrimitive.Trigger;

function TooltipContent({ className, sideOffset = 6, ...props }: ComponentProps<typeof TooltipPrimitive.Content>) {
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Content
        sideOffset={sideOffset}
        className={cn("z-[70] max-w-72 rounded-md bg-[var(--tooltip)] px-2.5 py-1.5 text-xs leading-5 text-[var(--tooltip-foreground)] shadow-[var(--shadow-overlay)]", className)}
        {...props}
      />
    </TooltipPrimitive.Portal>
  );
}

export { TooltipProvider, Tooltip, TooltipTrigger, TooltipContent };
