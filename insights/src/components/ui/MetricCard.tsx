import { metric } from "../../lib/format";
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
      <div className="text-xs font-semibold muted">{label}</div>
      <div
        className="mt-2 truncate text-[1.65rem] font-semibold tracking-[-.025em] tabular-nums"
        title={value === null ? "无有效样本" : String(value)}
      >
        {metric(value, unit)}
      </div>
      {detail && <div className="mt-1 text-xs muted">{detail}</div>}
      {formula && (
        <div className="mt-3 border-t border-[var(--border)] pt-2 text-[11px] muted">
          {formula}
        </div>
      )}
    </div>
  );
}
