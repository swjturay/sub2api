import { Check, ChevronRight, GitCompareArrows } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { metric } from "../../lib/format";
import type { ModelProfile } from "../../lib/types";
import { cn } from "../../lib/cn";
import { capabilityOptions, catalogPrice, modelIdentifier, modelVendor, supportsCapability, tokenLimit, vendorLabel } from "./modelCatalog";
import { PlatformMark } from "./PlatformMark";

export interface ModelCardProps {
  model: ModelProfile;
  selected: boolean;
  selectionDisabled: boolean;
  onToggle: () => void;
  onOpen: (opener: HTMLButtonElement) => void;
}

export function ModelCompareButton({ model, selected, selectionDisabled, onToggle }: Omit<ModelCardProps, "onOpen">) {
  return <Button variant={selected ? "primary" : "ghost"} className="h-8 px-2 text-xs" aria-label={(selected ? "取消选择 " : "选择 ") + model.name + " 对比"} aria-pressed={selected} disabled={selectionDisabled} onClick={onToggle}>
    {selected ? <Check aria-hidden="true" /> : <GitCompareArrows aria-hidden="true" />} {selected ? "已加入" : "对比"}
  </Button>;
}

export function ModelCard(props: ModelCardProps) {
  const { model, selected, onOpen } = props;
  const supported = capabilityOptions.filter(option => option.value !== "longContext" && supportsCapability(model, option.value));
  const vendor = modelVendor(model);
  return <article className={cn("catalog-card", selected && "is-selected")} aria-label={`${vendorLabel(vendor)} · ${model.name}`}>
    <div className="catalog-card-content">
      <div className="flex items-start gap-3">
        <PlatformMark platform={vendor} />
        <div className="min-w-0 flex-1">
          <p className="m-0 text-xs font-medium muted">{vendorLabel(vendor)}</p>
          <h2 className="mb-0 mt-1 break-words text-base font-semibold leading-6">{model.name}</h2>
          <p className="mb-0 mt-1 truncate font-mono text-xs muted" title={modelIdentifier(model)}>{modelIdentifier(model)}</p>
        </div>
      </div>
      <p className="catalog-description" title={model.description || undefined}>{model.description || "资料待完善，可在详情中查看已收录的规格。"}</p>
      <div className="catalog-card-tags" aria-label="模型能力与规格">
        <span className="catalog-context" title={model.contextLimit === null ? "上下文长度未知" : `${model.contextLimit.toLocaleString("zh-CN")} Tokens`}>{tokenLimit(model.contextLimit)} 上下文</span>
        {supported.map(option => <span className="catalog-capability" key={option.value}>{option.label}</span>)}
      </div>
      <div className="catalog-price-summary">
        <span className="text-xs muted">参考价格</span>
        <dl className="m-0 grid gap-1.5">
          {(["input", "output"] as const).map(kind => <div key={kind} className="flex flex-wrap items-baseline justify-between gap-x-2 gap-y-1">
            <dt className="text-xs muted">{kind === "input" ? "输入" : "输出"}</dt>
            <dd className="m-0 break-words text-xs font-semibold tabular-nums">{catalogPrice(model, kind)}</dd>
          </div>)}
        </dl>
      </div>
    </div>
    <dl className="catalog-card-metrics">
      {([["TPM", model.metrics.tpm.value, ""], ["RPM", model.metrics.rpm.value, ""], ["TTFT", model.metrics.ttft.value, "ms"], ["TPOT", model.metrics.tpot.value, "ms"]] as const).map(([label, value, unit]) => <div key={label}>
        <dt title={label === "TPOT" ? "估算每 Token 耗时" : label === "TTFT" ? "平均首字延迟" : `平均 ${label}`}>{label}</dt>
        <dd title={value === null ? "暂无有效样本" : `${value} ${unit}`}>{value === null ? "—" : metric(value, unit)}</dd>
      </div>)}
    </dl>
    <div className="flex items-center justify-between gap-2 px-3 py-2.5">
      <ModelCompareButton {...props} />
      <Button variant="ghost" onClick={event => onOpen(event.currentTarget)} className="h-8 px-2 text-xs">查看详情 <ChevronRight aria-hidden="true" /></Button>
    </div>
  </article>;
}
