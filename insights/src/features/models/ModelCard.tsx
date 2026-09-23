import { Check, ChevronRight } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { metric } from "../../lib/format";
import type { ModelProfile } from "../../lib/types";
import { cn } from "../../lib/cn";

const capabilities = [
  ["reasoning", "推理"],
  ["toolCalling", "工具调用"],
  ["structuredOutput", "结构化输出"],
] as const;

function hasProfile(model: ModelProfile) {
  return Boolean(
    model.description || model.useCases.length || model.contextLimit !== null ||
    model.maxOutput !== null || model.inputModalities.length || model.outputModalities.length ||
    model.reasoning !== "unknown" || model.toolCalling !== "unknown" ||
    model.structuredOutput !== "unknown" || model.sources.length,
  );
}

export function ModelCard({ model, selected, selectionDisabled, onToggle, onOpen }: {
  model: ModelProfile;
  selected: boolean;
  selectionDisabled: boolean;
  onToggle: () => void;
  onOpen: (opener: HTMLButtonElement) => void;
}) {
  const knownCapabilities = capabilities.filter(([key]) => model[key] === "supported");
  const profileAvailable = hasProfile(model);
  const metrics = [
    ["平均 TPM", model.metrics.tpm.value, ""],
    ["平均 RPM", model.metrics.rpm.value, ""],
    ["平均 TTFT", model.metrics.ttft.value, "ms"],
    ["估算 TPOT", model.metrics.tpot.value, "ms"],
  ] as const;
  return (
    <article className={cn(
      "flex min-h-[286px] min-w-0 flex-col overflow-hidden rounded-[12px] border bg-[var(--surface)] shadow-[var(--shadow-panel)] transition-[border-color,box-shadow,background-color]",
      selected ? "border-[var(--primary)] shadow-[0_0_0_2px_var(--primary-soft)]" : "border-[var(--border)] hover:border-[var(--border-strong)]",
    )}>
      <div className="flex flex-1 flex-col p-4">
        <div className="flex min-w-0 items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="m-0 truncate text-xs font-medium muted" title={model.platform}>{model.platform}</p>
            <h2 className="mt-1 mb-0 [overflow-wrap:anywhere] text-base font-semibold leading-6" title={model.name}>{model.name}</h2>
          </div>
          <button
            type="button"
            aria-label={(selected ? "取消选择 " : "选择 ") + model.name + " 对比"}
            aria-pressed={selected}
            disabled={selectionDisabled}
            onClick={onToggle}
            className={cn(
              "grid h-9 w-9 shrink-0 place-items-center rounded-[9px] border transition-[border-color,background-color,color] disabled:cursor-not-allowed disabled:opacity-40",
              selected ? "border-[var(--primary)] bg-[var(--primary)] text-[var(--primary-foreground)]" : "border-[var(--border)] bg-[var(--surface)] text-transparent hover:border-[var(--primary)]",
            )}
          >
            <Check className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
        <div className="mt-3 min-h-12">
          {model.description ? (
            <p className="m-0 line-clamp-2 text-sm leading-6 muted" title={model.description}>{model.description}</p>
          ) : !profileAvailable ? (
            <span className="inline-flex rounded-full border border-[var(--border)] bg-[var(--surface-subtle)] px-2 py-1 text-xs muted">资料待完善</span>
          ) : (
            <span className="text-xs muted">暂无介绍</span>
          )}
        </div>
        {knownCapabilities.length > 0 && (
          <div className="mt-3 flex min-h-6 flex-wrap gap-1.5" aria-label="已知支持能力">
            {knownCapabilities.map(([, label]) => (
              <span key={label} className="rounded-full bg-[var(--primary-soft)] px-2 py-1 text-xs font-medium text-[var(--primary)]">{label}</span>
            ))}
          </div>
        )}
      </div>
      <dl className="grid grid-cols-4 border-y border-[var(--border)] bg-[var(--surface-subtle)]">
        {metrics.map(([label, value, unit], index) => (
          <div key={label} className={cn("min-w-0 px-2 py-3", index > 0 && "border-l border-[var(--border)]")}>
            <dt className="truncate text-xs muted" title={label}>{label}</dt>
            <dd className="m-0 mt-1 truncate text-xs font-semibold tabular-nums" title={value === null ? "暂无数据" : String(value) + " " + unit}>{value === null ? "—" : metric(value, unit)}</dd>
          </div>
        ))}
      </dl>
      <div className="flex items-center justify-end px-3 py-2">
        <Button variant="ghost" onClick={(event) => onOpen(event.currentTarget)} className="h-8 px-2 font-medium">
          查看详情 <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </article>
  );
}
