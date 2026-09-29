import { useMemo, useRef, useState } from "react";
import { Search, X, ArrowLeft, ArrowRight } from "lucide-react";
import { Input, Segmented, Select } from "../components/ui/Controls";
import { Empty, ErrorBanner } from "../components/ui/States";
import { Button } from "../components/ui/Button";
import { CatalogFilters } from "../features/models/CatalogFilters";
import { ModelTable } from "../features/models/ModelTable";
import { emptyCatalogFilters, filterCatalog, modelVendor, type CatalogSort } from "../features/models/modelCatalog";
import { ModelCard } from "../features/models/ModelCard";
import { ModelComparisonDialog } from "../features/models/ModelComparisonDialog";
import { ModelDetailDialog } from "../features/models/ModelDetailDialog";
import { ModelSelectionTray } from "../features/models/ModelSelectionTray";
import { insightsApi } from "../lib/api";
import type { ModelProfile } from "../lib/types";
import { useRemote } from "../lib/useRemote";
import { cn } from "../lib/cn";

export function ModelsPage({ auto }: { auto: boolean }) {
  const window = "24h";
  const [filters, setFilters] = useState(emptyCatalogFilters);
  const [sort, setSort] = useState<CatalogSort>("rpm");
  const [view, setView] = useState<"cards" | "table">("cards");
  const [page, setPage] = useState(1);
  const resultsRef = useRef<HTMLDivElement | null>(null);
  const updateFilters = (next: typeof filters) => { setFilters(next); setPage(1); };
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
  const models = state.data || [];
  const filtered = useMemo(() => filterCatalog(state.data || [], filters, sort), [state.data, filters, sort]);
  const pageCount = Math.max(1, Math.ceil(filtered.length / 12));
  const currentPage = Math.min(page, pageCount);
  const visible = filtered.slice((currentPage - 1) * 12, currentPage * 12);
  const vendors = new Set(models.map(modelVendor)).size;
  const changePage = (next: number) => { setPage(next); resultsRef.current?.scrollIntoView({ block: "start" }); };
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
      <header className="catalog-header">
        <div className="catalog-heading-row">
          <div>
            <h1 className="m-0 text-3xl font-semibold tracking-tight">模型广场</h1>
            <p className="mb-0 mt-2 text-sm muted">发现适合任务的模型，对比能力、参考价格与实测性能。</p>
          </div>
          <dl className="catalog-overview">
            {[["收录模型", state.data ? models.length : "—"], ["模型厂商", state.data ? vendors : "—"]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </div>
        <label className="catalog-search">
          <span className="sr-only">搜索模型</span>
          <Search className="size-5 shrink-0 muted" aria-hidden="true" />
          <Input id="model-catalog-search" className="!h-11 !border-0 !bg-transparent !px-0 !shadow-none" value={filters.search} onChange={event => updateFilters({ ...filters, search: event.target.value })} placeholder="搜索模型名称、厂商或用途…" />
          {filters.search && <Button variant="ghost" className="w-8 px-0" aria-label="清除搜索" onClick={() => updateFilters({ ...filters, search: "" })}><X aria-hidden="true" /></Button>}
        </label>
      </header>

      {state.error && <ErrorBanner message={state.error} retry={state.refresh} stale={!!state.data} />}
      {detailError && <ErrorBanner message={detailError.message} retry={() => void openDetail(detailError.id)} />}
      {detailLoading && <div role="status" className="rounded-[10px] border border-[var(--border)] bg-[var(--surface-subtle)] px-3 py-2 text-sm muted">正在加载模型详情…</div>}

      <div className="catalog-layout">
        <CatalogFilters models={models} value={filters} onChange={updateFilters} onReset={() => updateFilters(emptyCatalogFilters)} />
        <div ref={resultsRef} className="catalog-results min-w-0">
          <div className="catalog-toolbar">
            <div className="min-w-0 flex-1">
              <h2 className="m-0 text-base font-semibold">模型目录 <span className="ml-1 text-sm font-normal muted" role="status" aria-live="polite">{state.data ? `${filtered.length} / ${models.length}` : "加载中"}</span></h2>
              <p className="mb-0 mt-1 text-xs muted">选择 2–4 个模型进行对比</p>
            </div>
            <label className="grid gap-1 text-xs muted">排序
              <Select aria-label="模型排序" value={sort} onChange={event => { setSort(event.target.value as CatalogSort); setPage(1); }}>
                <option value="rpm">调用频率优先</option><option value="name">名称 A–Z</option><option value="context">上下文从大到小</option><option value="latency">首字延迟从低到高</option>
              </Select>
            </label>
            <div className="grid gap-1"><span className="text-xs muted">展示方式</span><Segmented value={view} onChange={setView} label="模型展示方式" options={[{ value: "cards", label: "卡片" }, { value: "table", label: "列表" }]} /></div>
          </div>
          {state.loading && !state.data ? <div className="catalog-grid" aria-label="正在加载模型目录">
            {Array.from({ length: 6 }, (_, index) => <div key={index} className="h-[350px] animate-pulse rounded-[12px] border border-[var(--border)] bg-[var(--surface)]" />)}
          </div> : filtered.length ? <>
            {view === "cards" ? <div className="catalog-grid">
              {visible.map(model => <ModelCard key={model.id} model={model} selected={selected.includes(model.id)} selectionDisabled={!selected.includes(model.id) && selected.length >= 4} onToggle={() => toggle(model.id)} onOpen={opener => { detailOpener.current = opener; void openDetail(model.id); }} />)}
            </div> : <ModelTable models={visible} selected={selected} toggle={toggle} open={(model, opener) => { detailOpener.current = opener; void openDetail(model.id); }} />}
            <nav className="catalog-pagination" aria-label="模型目录分页">
              <span className="text-xs muted">显示 {(currentPage - 1) * 12 + 1}–{Math.min(currentPage * 12, filtered.length)} 项，共 {filtered.length} 项</span>
              <div className="flex items-center gap-3">
                <Button variant="secondary" disabled={currentPage <= 1} onClick={() => changePage(currentPage - 1)} aria-label="上一页模型"><ArrowLeft aria-hidden="true" /></Button>
                <span className="text-sm tabular-nums">{currentPage} / {pageCount}</span>
                <Button variant="secondary" disabled={currentPage >= pageCount} onClick={() => changePage(currentPage + 1)} aria-label="下一页模型"><ArrowRight aria-hidden="true" /></Button>
              </div>
            </nav>
          </> : <div className="panel text-center"><Empty title="没有匹配模型" detail="试试其他关键词，或减少筛选条件。" /><Button variant="secondary" onClick={() => updateFilters(emptyCatalogFilters)}>清除筛选</Button></div>}
        </div>
      </div>

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
          key={detail.id}
          model={detail}
          close={() => setDetail(null)}
          restoreFocusElement={detailOpener.current}
        />
      )}
    </div>
  );
}
