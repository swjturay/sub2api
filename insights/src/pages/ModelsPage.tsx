import { useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";
import { Input, Segmented } from "../components/ui/Controls";
import { Empty, ErrorBanner } from "../components/ui/States";
import { ModelCard } from "../features/models/ModelCard";
import { ModelComparisonDialog } from "../features/models/ModelComparisonDialog";
import { ModelDetailDialog } from "../features/models/ModelDetailDialog";
import { ModelSelectionTray } from "../features/models/ModelSelectionTray";
import { insightsApi } from "../lib/api";
import type { ModelProfile } from "../lib/types";
import { useRemote } from "../lib/useRemote";
import { cn } from "../lib/cn";

export function ModelsPage({ auto }: { auto: boolean }) {
  const [window, setWindow] = useState<"24h" | "7d">("24h");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [compareOpen, setCompareOpen] = useState(false);
  const [detail, setDetail] = useState<ModelProfile | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<{ message: string; id: string } | null>(null);
  const comparisonOpener = useRef<HTMLElement | null>(null);
  const detailOpener = useRef<HTMLElement | null>(null);

  const state = useRemote((signal) => insightsApi.models(window, signal), [window], auto);
  const comparison = useRemote(
    (signal) => compareOpen && selected.length >= 2
      ? insightsApi.compareModels(selected, window, signal)
      : Promise.resolve(null),
    [compareOpen, selected.join("|"), window],
    auto && compareOpen,
  );
  const filtered = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    if (!query) return state.data || [];
    return (state.data || []).filter((model) => [model.name, model.platform, model.description || "", ...model.useCases]
      .join(" ")
      .toLocaleLowerCase()
      .includes(query));
  }, [state.data, search]);
  const selectedModels = useMemo(() => selected
    .map((id) => state.data?.find((model) => model.id === id))
    .filter((model): model is ModelProfile => Boolean(model)), [selected, state.data]);

  const toggle = (id: string) => {
    setSelected((current) => {
      const next = current.includes(id)
        ? current.filter((item) => item !== id)
        : current.length < 4 ? [...current, id] : current;
      if (next.length < 2) setCompareOpen(false);
      return next;
    });
  };
  const openDetail = async (id: string) => {
    setDetailLoading(true);
    setDetailError(null);
    try {
      setDetail(await insightsApi.model(id, window));
    } catch (caught) {
      setDetailError({ message: (caught as Error).message, id });
    } finally {
      setDetailLoading(false);
    }
  };

  return (
    <div className={cn("page-stack grid min-w-0 gap-5", selected.length > 0 && "pb-32 sm:pb-24")}>
      <div className="page-heading flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="m-0 text-2xl font-semibold">模型广场</h1>
          <p className="mb-0 mt-1 text-sm muted">系统配置目录与全平台聚合性能；目录存在不代表实时在线。</p>
        </div>
        <Segmented
          value={window}
          onChange={setWindow}
          label="性能窗口"
          options={[{ value: "24h", label: "滚动 24 小时" }, { value: "7d", label: "滚动 7 天" }]}
        />
      </div>

      <div className="flex flex-wrap items-center gap-3 rounded-[12px] border border-[var(--border)] bg-[var(--surface)] p-3 shadow-[var(--shadow-sm)]">
        <label className="relative min-w-[240px] flex-1">
          <span className="sr-only">搜索模型</span>
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 muted" aria-hidden="true" />
          <Input id="model-catalog-search" className="w-full pl-9" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索模型、平台或用途" />
        </label>
        <div className="ml-auto flex items-center gap-3 text-xs muted">
          <span>{filtered.length} 个模型</span>
          <span className="hidden h-4 w-px bg-[var(--border)] sm:block" aria-hidden="true" />
          <span className="hidden sm:inline">选择 2–4 个进行对比</span>
        </div>
      </div>

      {state.error && <ErrorBanner message={state.error} retry={state.refresh} stale={!!state.data} />}
      {detailError && <ErrorBanner message={detailError.message} retry={() => void openDetail(detailError.id)} />}
      {detailLoading && <div role="status" className="rounded-[10px] border border-[var(--border)] bg-[var(--surface-subtle)] px-3 py-2 text-sm muted">正在加载模型详情…</div>}

      {state.loading && !state.data ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4" aria-label="正在加载模型目录">
          {Array.from({ length: 6 }, (_, index) => <div key={index} className="h-[286px] animate-pulse rounded-[12px] border border-[var(--border)] bg-[var(--surface)]" />)}
        </div>
      ) : filtered.length ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
          {filtered.map((model) => (
            <ModelCard
              key={model.id}
              model={model}
              selected={selected.includes(model.id)}
              selectionDisabled={!selected.includes(model.id) && selected.length >= 4}
              onToggle={() => toggle(model.id)}
              onOpen={(opener) => { detailOpener.current = opener; void openDetail(model.id); }}
            />
          ))}
        </div>
      ) : (
        <div className="rounded-[12px] border border-[var(--border)] bg-[var(--surface)]">
          <Empty title="没有匹配模型" detail="请调整搜索关键词；系统目录不会按当前用户权限或分组缩减。" />
        </div>
      )}

      <ModelSelectionTray selected={selectedModels} onRemove={toggle} onClear={() => { setSelected([]); setCompareOpen(false); }} onCompare={(opener) => { comparisonOpener.current = opener; setCompareOpen(true); }} />
      <ModelComparisonDialog
        open={compareOpen}
        onOpenChange={setCompareOpen}
        data={comparison.data}
        loading={comparison.loading}
        error={comparison.error}
        retry={comparison.refresh}
        remove={toggle}
        window={window}
        restoreFocusElement={comparisonOpener.current}
      />
      {detail && (
        <ModelDetailDialog
          model={detail}
          close={() => setDetail(null)}
          restoreFocusElement={detailOpener.current}
        />
      )}
    </div>
  );
}
