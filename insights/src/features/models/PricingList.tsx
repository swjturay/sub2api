import type { ModelProfile } from "../../lib/types";
import { formatPricingEntry } from "./modelPricing";

export function PricingList({ pricing, compact = false }: {
  pricing: ModelProfile["pricing"];
  compact?: boolean;
}) {
  if (!pricing.length) return <span className="muted">未配置</span>;
  return (
    <div className="grid gap-2">
      {pricing.map((entry, index) => {
        const formatted = formatPricingEntry(entry);
        const tooltip = [
          "原始价格：" + formatted.rawValue,
          formatted.rawCondition && "原始条件：" + formatted.rawCondition,
        ].filter(Boolean).join("\n");
        return (
          <div
            key={[entry.label, entry.value, entry.condition || index].join("-")}
            className={compact ? "grid gap-1" : "rounded-[8px] border border-[var(--border)] bg-[var(--surface-subtle)] p-3"}
            title={tooltip}
          >
            <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
              <span className="text-xs font-medium muted">{formatted.label}</span>
              <strong className="text-sm font-semibold tabular-nums">{formatted.displayValue}</strong>
            </div>
            {formatted.conditions.length > 0 && (
              <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs muted">
                {formatted.conditions.map((condition, conditionIndex) => (
                  <span key={condition.label + "-" + conditionIndex}>
                    {condition.label}：<span className="tabular-nums">{condition.value}</span>
                  </span>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
