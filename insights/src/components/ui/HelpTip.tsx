import { CircleHelp } from "lucide-react";
import type { ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "./Tooltip";

export function HelpTip({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" className="help-tip" aria-label={label}>
          <CircleHelp aria-hidden="true" />
        </button>
      </TooltipTrigger>
      <TooltipContent className="max-w-80">{children}</TooltipContent>
    </Tooltip>
  );
}
