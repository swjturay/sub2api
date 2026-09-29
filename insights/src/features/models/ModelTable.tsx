import { Button } from "../../components/ui/Button";
import { metric } from "../../lib/format";
import type { ModelProfile } from "../../lib/types";
import { ModelCompareButton } from "./ModelCard";
import { catalogPrice, modelIdentifier, modelVendor, tokenLimit, vendorLabel } from "./modelCatalog";
import { PlatformMark } from "./PlatformMark";

export function ModelTable({ models, selected, toggle, open }: {
  models: ModelProfile[]; selected: string[]; toggle: (id: string) => void;
  open: (model: ModelProfile, opener: HTMLButtonElement) => void;
}) {
  return <div className="catalog-table-wrap"><table className="catalog-table">
    <caption className="sr-only">模型规格、参考价格与性能，排序由上方模型排序控件控制</caption>
    <thead><tr>{["模型 / 厂商", "上下文", "参考价格", "平均 RPM", "平均 TTFT", "操作"].map(title => <th scope="col" key={title}>{title}</th>)}</tr></thead>
    <tbody>{models.map(model => <tr key={model.id}>
      <th scope="row"><div className="flex items-center gap-3"><PlatformMark platform={modelVendor(model)} /><div className="min-w-0"><span className="break-words font-semibold">{model.name}</span><span className="mt-1 block break-all font-mono text-xs font-normal muted">{modelIdentifier(model)}</span><span className="mt-1 block text-xs font-normal muted">{vendorLabel(modelVendor(model))}</span></div></div></th>
      <td title={model.contextLimit === null ? "未知" : String(model.contextLimit)}>{tokenLimit(model.contextLimit)}</td>
      <td><div className="grid gap-1 text-xs"><span>输入 {catalogPrice(model, "input")}</span><span>输出 {catalogPrice(model, "output")}</span></div></td>
      <td>{model.metrics.rpm.value === null ? "—" : metric(model.metrics.rpm.value)}</td>
      <td>{model.metrics.ttft.value === null ? "—" : metric(model.metrics.ttft.value, "ms")}</td>
      <td><div className="flex flex-wrap gap-1"><ModelCompareButton model={model} selected={selected.includes(model.id)} selectionDisabled={!selected.includes(model.id) && selected.length >= 4} onToggle={() => toggle(model.id)} /><Button variant="ghost" className="h-8 px-2 text-xs" onClick={event => open(model, event.currentTarget)}>查看详情</Button></div></td>
    </tr>)}</tbody>
  </table></div>;
}
