import { useMemo, useState } from "react";
import { ArrowUpRight, Search } from "lucide-react";
import type { ModelSlice } from "../../lib/types";
import { Chart, palette } from "./Chart";
import { chartColors, chartFontFamily, fullNumber, tooltipTheme } from "./chartTheme";
import { metric, percent } from "../../lib/format";
import { Button } from "../ui/Button";
import { Input } from "../ui/Controls";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "../ui/Dialog";

function modelIdentity(model: ModelSlice) {
  const separator = model.modelId.indexOf(":");
  const platform = model.platform || (separator > 0 ? model.modelId.slice(0, separator) : "");
  const name = model.name === model.modelId && separator > 0 ? model.name.slice(separator + 1) : model.name;
  return { name, platform: platform === "unknown" ? "平台未知" : platform };
}
const escapeText = (value: string) => value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;");

export function ModelDonut({ models, compact = false }: { models: ModelSlice[]; compact?: boolean }) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const sorted = useMemo(() => [...models].sort((a, b) => b.requests - a.requests), [models]);
  const total = sorted.reduce((sum, model) => sum + model.requests, 0);
  const leading = sorted.slice(0, 5);
  const remaining = sorted.slice(5);
  const remainingRequests = remaining.reduce((sum, model) => sum + model.requests, 0);
  const chartData = [...leading.map(model => ({ name: modelIdentity(model).name, value: model.requests })), ...(remaining.length ? [{ name: "其他", value: remainingRequests, itemStyle: { color: "#94a3b8" } }] : [])];
  const colors = chartColors();
  const visible = sorted.filter(model => [model.name, model.modelId, model.platform].join(" ").toLowerCase().includes(search.toLowerCase()));
  const share = (requests: number) => total > 0 ? requests / total : null;
  const table = (items: ModelSlice[]) => <table className="w-full text-sm">
    <thead className="sticky top-0 z-10 bg-[var(--surface)] text-left"><tr>
      <th className="py-3 pr-4 font-medium">模型</th><th className="pr-4 font-medium">平台</th><th className="pr-4 text-right font-medium">请求次数</th><th className="text-right font-medium">占比</th>
    </tr></thead>
    <tbody>{items.map(model => {
      const identity = modelIdentity(model);
      const rank = sorted.indexOf(model);
      return <tr key={model.modelId} className="group">
        <td className="max-w-[280px] border-t border-[var(--border)] py-3 pr-4 font-medium"><span className="flex items-baseline gap-2" title={model.modelId}><span aria-hidden className="inline-block size-2 shrink-0 rounded-sm" style={{ background: rank < 5 ? palette[rank] : "#94a3b8" }} /><span className="break-words">{identity.name}</span></span></td>
        <td className="border-t border-[var(--border)] pr-4 text-xs muted">{identity.platform || "—"}</td>
        <td className="border-t border-[var(--border)] pr-4 text-right tabular-nums">{fullNumber(model.requests)}</td>
        <td className="border-t border-[var(--border)] text-right tabular-nums muted">{percent(share(model.requests))}</td>
      </tr>;
    })}</tbody>
  </table>;
  const donut = <Chart label="模型调用分布" height={250} option={{
    color: palette,
    tooltip: { ...tooltipTheme(), trigger: "item", formatter: (parameter: unknown) => {
      const item = parameter as { name: string; value: number };
      return escapeText(item.name) + "<br/><b>" + fullNumber(item.value) + "</b> 次 · " + percent(share(item.value));
    } },
    legend: { show: false },
    series: [{ type: "pie", radius: ["60%", "83%"], center: ["50%", "50%"], label: { show: false }, itemStyle: { borderColor: colors.surface, borderWidth: 3, borderRadius: 4 }, emphasis: { scaleSize: 3 }, data: chartData }],
    graphic: [{ type: "text", left: "center", top: "43%", style: { text: metric(total), fill: colors.ink, fontSize: 27, fontWeight: 600, fontFamily: chartFontFamily() } }, { type: "text", left: "center", top: "58%", style: { text: "总调用次数", fill: colors.muted, fontSize: 12, fontFamily: chartFontFamily() } }],
  }} />;
  if (!compact) return <div className="grid items-center gap-6 lg:grid-cols-[minmax(220px,36%)_1fr]">{donut}<div className="max-h-72 overflow-auto scrollbar-thin">{table(sorted)}</div></div>;
  return <Dialog open={open} onOpenChange={setOpen}>
    <div className="flex items-baseline gap-2 pb-4 pt-1"><strong className="text-2xl font-semibold tracking-tight tabular-nums">{metric(total)}</strong><span className="text-xs muted">次调用 · {sorted.length} 个模型</span></div>
    <ol className="m-0 grid list-none gap-3 p-0" aria-label="模型调用排名">
      {leading.map((model, index) => {
        const identity = modelIdentity(model);
        return <li key={model.modelId}>
          <div className="mb-1.5 flex min-w-0 items-baseline justify-between gap-3 text-xs"><span className="min-w-0 truncate font-medium" title={identity.platform + " · " + identity.name}>{identity.name}</span><span className="shrink-0 tabular-nums muted" title={fullNumber(model.requests) + " 次"}>{percent(share(model.requests))}</span></div>
          <div className="h-1 overflow-hidden rounded-full bg-[var(--surface-muted)]" aria-hidden><div className="h-full rounded-full" style={{ width: (share(model.requests) ?? 0) * 100 + "%", background: palette[index] }} /></div>
        </li>;
      })}
      {remaining.length > 0 && <li className="flex items-center justify-between gap-3 text-xs muted"><span>其他 {remaining.length} 个模型</span><span className="tabular-nums" title={fullNumber(remainingRequests) + " 次"}>{percent(share(remainingRequests))}</span></li>}
    </ol>
    <DialogTrigger asChild><Button className="mt-4 w-full justify-between !px-0 text-xs" variant="ghost" onClick={() => setSearch("")}>查看全部模型<ArrowUpRight className="h-3.5 w-3.5" /></Button></DialogTrigger>
      <DialogContent className="max-w-[960px]">
        <DialogHeader><DialogTitle>模型调用分布</DialogTitle><DialogDescription>当前范围内共 {sorted.length} 个模型，按请求次数排序。</DialogDescription></DialogHeader>
        <div className="relative mt-2"><Search aria-hidden className="pointer-events-none absolute left-3 top-3 h-4 w-4 muted" /><Input aria-label="搜索模型调用分布" placeholder="搜索模型或平台" className="w-full pl-9" value={search} onChange={event => setSearch(event.target.value)} /></div>
        <div className="grid min-h-0 gap-5 lg:grid-cols-[240px_minmax(0,1fr)]"><div>{donut}</div><div className="max-h-[52vh] min-w-0 overflow-auto scrollbar-thin" tabIndex={0} aria-label="全部模型调用数据">{visible.length ? table(visible) : <p className="py-10 text-center text-sm muted">没有匹配的模型</p>}</div></div>
      </DialogContent>
  </Dialog>;
}
