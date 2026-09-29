import { SlidersHorizontal } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { cn } from "../../lib/cn";
import type { ModelProfile } from "../../lib/types";
import { capabilityOptions, modalityLabel, modelVendor, vendorLabel, type CatalogFilters as Filters } from "./modelCatalog";

export function CatalogFilters({ models, value, onChange, onReset }: {
  models: ModelProfile[]; value: Filters; onChange: (value: Filters) => void; onReset: () => void;
}) {
  const vendors = [...new Set(models.map(modelVendor))].sort();
  const inputs = [...new Set(models.flatMap(model => model.inputModalities))].sort();
  const active = Boolean(value.search || value.vendor || value.input || value.capabilities.length);
  return <aside className="catalog-filters panel" aria-label="模型分类筛选">
    <div className="flex items-center justify-between gap-2">
      <h2 className="m-0 flex items-center gap-2 text-sm font-semibold"><SlidersHorizontal className="size-4" aria-hidden="true" />筛选模型</h2>
      <Button variant="ghost" className="h-8 px-2 text-xs" onClick={onReset} disabled={!active}>重置</Button>
    </div>
    <fieldset className="catalog-filter-section">
      <legend>模型厂商</legend>
      <div className="catalog-platforms">
        {[{ id: "", label: "全部厂商", count: models.length }, ...vendors.map(id => ({ id, label: vendorLabel(id), count: models.filter(model => modelVendor(model) === id).length }))].map(option => <button
          key={option.id} type="button" aria-label={`${option.label} ${option.count}`} aria-pressed={value.vendor === option.id}
          onClick={() => onChange({ ...value, vendor: option.id })}
          className={cn("catalog-platform", value.vendor === option.id && "is-selected")}
        ><span>{option.label}</span><span className="catalog-filter-count">{option.count}</span></button>)}
      </div>
    </fieldset>
    <fieldset className="catalog-filter-section">
      <legend>模型能力 <span className="font-normal muted">· 满足全部所选项</span></legend>
      <div className="grid gap-3">
        {capabilityOptions.map(option => <label className="flex cursor-pointer items-center gap-2 text-sm" key={option.value}>
          <input type="checkbox" className="size-4 accent-[var(--primary)]" checked={value.capabilities.includes(option.value)} onChange={() => onChange({ ...value, capabilities: value.capabilities.includes(option.value) ? value.capabilities.filter(item => item !== option.value) : [...value.capabilities, option.value] })} />
          {option.label}
        </label>)}
      </div>
    </fieldset>
    <fieldset className="catalog-filter-section">
      <legend>输入类型</legend>
      <div className="flex flex-wrap gap-2">
        {[{ id: "", label: "全部" }, ...inputs.map(id => ({ id, label: modalityLabel(id) }))].map(option => <button type="button" key={option.id} aria-pressed={value.input === option.id} className={cn("catalog-filter-chip", value.input === option.id && "is-selected")} onClick={() => onChange({ ...value, input: option.id })}>{option.label}</button>)}
      </div>
    </fieldset>
  </aside>;
}
