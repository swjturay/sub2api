import { metric } from "../../lib/format";
import { CircleHelp } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "./Tooltip";
export function MetricCard({
  label,
  value,
  unit,
  detail,
  formula,
}: {
  label: string;
  value: number | null;
  unit?: string;
  detail?: string;
  formula?: string;
}) {
  return (
    <div className="metric-card min-w-0">
      <div className="metric-card__label">
        <span>{label}</span>
        {formula && <Tooltip><TooltipTrigger asChild><button type="button" className="metric-card__formula" aria-label={`${label}计算方法`}><CircleHelp aria-hidden="true" /></button></TooltipTrigger><TooltipContent>{formula}</TooltipContent></Tooltip>}
      </div>
      <div
        className="metric-card__value tabular-nums"
        title={value === null ? "未知或无有效样本" : String(value)}
        aria-label={value === null ? `${label}：未知或无有效样本` : undefined}
      >
        {metric(value)}{value !== null && unit && <span className="metric-card__unit">{unit}</span>}
      </div>
      {detail && <div className="metric-card__detail">{detail}</div>}
    </div>
  );
}
