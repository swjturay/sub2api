import { GitCompareArrows, X } from "lucide-react";
import { Button } from "../../components/ui/Button";
import type { ModelProfile } from "../../lib/types";

type SelectedModel = Pick<ModelProfile, "id" | "name" | "platform">;

export function ModelSelectionTray({ selected, onRemove, onClear, onCompare }: {
  selected: SelectedModel[];
  onRemove: (id: string) => void;
  onClear: () => void;
  onCompare: (opener: HTMLButtonElement) => void;
}) {
  if (!selected.length) return null;
  const ready = selected.length >= 2;
  return (
    <aside
      aria-label="模型对比选择"
      className="fixed inset-x-4 bottom-4 z-30 mx-auto flex w-[min(calc(100%_-_2rem),72rem)] flex-wrap items-center gap-3 rounded-[12px] border border-[var(--border-strong)] bg-[var(--surface-raised)] p-3 shadow-[var(--shadow-overlay)]"
    >
      <div className="flex min-w-[150px] items-center gap-2">
        <GitCompareArrows className="h-4 w-4 text-[var(--primary)]" aria-hidden="true" />
        <div>
          <div className="text-sm font-semibold">模型对比</div>
          <div className="text-xs muted" aria-live="polite">已选择 {selected.length}/4{ready ? "" : "，还需选择 1 个"}</div>
        </div>
      </div>
      <div className="flex min-w-0 flex-1 flex-wrap gap-2">
        {selected.map((model) => (
          <span key={model.id} className="inline-flex min-w-0 max-w-[220px] items-center gap-1 rounded-[8px] border border-[var(--border)] bg-[var(--surface-subtle)] py-1 pl-2.5 pr-1 text-xs">
            <span className="truncate" title={model.platform + " · " + model.name}>{model.name}</span>
            <button
              type="button"
              className="grid h-7 w-7 shrink-0 place-items-center rounded-[6px] muted hover:bg-[var(--surface-muted)] hover:text-[var(--ink)]"
              aria-label={"移除 " + model.name}
              onClick={() => onRemove(model.id)}
            >
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          </span>
        ))}
      </div>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" onClick={onClear}>清空</Button>
        <Button disabled={!ready} onClick={(event) => onCompare(event.currentTarget)}>
          <GitCompareArrows aria-hidden="true" /> 对比 {selected.length} 个模型
        </Button>
      </div>
    </aside>
  );
}
