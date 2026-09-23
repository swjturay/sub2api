import type { ReactNode } from "react";
import { X } from "lucide-react";
import { Chart } from "../../components/charts/Chart";
import { Button } from "../../components/ui/Button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/Dialog";
import { ErrorBanner, Loading } from "../../components/ui/States";
import { buildComparisonTrendOption } from "../chartOptions";
import type { Capability, ModelComparison, ModelProfile } from "../../lib/types";
import { PricingList } from "./PricingList";
import { restoreModelDialogFocus } from "./modelDialogFocus";

const capability = (value: Capability) => value === "supported" ? "支持" : value === "unsupported" ? "不支持" : "未知";
const amount = (value: number | null) => value === null ? "未知" : value.toLocaleString("zh-CN");

function Sources({ model }: { model: ModelProfile }) {
  if (!model.sources.length) return <span className="muted">未配置</span>;
  return (
    <div className="grid gap-1">
      {model.sources.map((source, index) => (
        <span key={source.url + "-" + index} className="break-words">
          {source.label || source.url}{source.updatedAt ? " · " + source.updatedAt.slice(0, 10) : ""}
        </span>
      ))}
    </div>
  );
}

export function ModelComparisonDialog({ open, onOpenChange, data, loading, error, retry, remove, window, restoreFocusElement }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  data: ModelComparison | null;
  loading: boolean;
  error: string | null;
  retry: () => void;
  remove: (id: string) => void;
  window: "24h" | "7d";
  restoreFocusElement: HTMLElement | null;
}) {
  const rows: Array<{ label: string; render: (model: ModelProfile) => ReactNode }> = [
    { label: "介绍", render: (model) => model.description || <span className="muted">暂无介绍</span> },
    { label: "适用场景", render: (model) => model.useCases.join("、") || <span className="muted">未知</span> },
    { label: "上下文", render: (model) => amount(model.contextLimit) },
    { label: "最大输出", render: (model) => amount(model.maxOutput) },
    { label: "输入模态", render: (model) => model.inputModalities.join("、") || <span className="muted">未知</span> },
    { label: "输出模态", render: (model) => model.outputModalities.join("、") || <span className="muted">未知</span> },
    { label: "推理", render: (model) => capability(model.reasoning) },
    { label: "工具调用", render: (model) => capability(model.toolCalling) },
    { label: "结构化输出", render: (model) => capability(model.structuredOutput) },
    { label: "参考价格", render: (model) => <PricingList pricing={model.pricing} compact /> },
    { label: "资料来源", render: (model) => <Sources model={model} /> },
  ];
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="flex max-h-[calc(100vh-32px)] w-[min(calc(100vw-24px),78rem)] max-w-[78rem] flex-col overflow-hidden p-0"
        onCloseAutoFocus={(event) => restoreModelDialogFocus(event, restoreFocusElement)}
      >
        <DialogHeader className="border-b border-[var(--border)] px-6 py-5">
          <DialogTitle>模型对比</DialogTitle>
          <DialogDescription>并排查看 2–4 个模型的资料、参考价格与当前性能趋势。</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-auto p-5 sm:p-6">
          {error && <ErrorBanner message={error} retry={retry} stale={!!data} />}
          {loading && !data ? <Loading label="正在加载模型对比" /> : data ? (
            <div className="grid gap-7">
              <div className="max-w-full overflow-auto rounded-[10px] border border-[var(--border)]">
                <table className="w-full text-sm" style={{ minWidth: 180 + data.models.length * 250 }}>
                  <thead className="sticky top-0 z-20 bg-[var(--surface-raised)]">
                    <tr>
                      <th scope="col" className="sticky left-0 z-30 w-[180px] border-r border-[var(--border)] bg-[var(--surface-raised)] px-4 text-left">项目</th>
                      {data.models.map((model) => (
                        <th scope="col" key={model.id} className="min-w-[250px] border-r border-[var(--border)] px-4 text-left last:border-r-0">
                          <div className="flex items-start justify-between gap-2 py-1">
                            <div className="min-w-0">
                              <div className="truncate text-sm font-semibold text-[var(--ink)]" title={model.name}>{model.name}</div>
                              <div className="mt-0.5 truncate text-xs font-normal muted" title={model.platform}>{model.platform}</div>
                            </div>
                            <Button variant="ghost" className="h-8 w-8 p-0" aria-label={"移除 " + model.name} onClick={() => remove(model.id)}>
                              <X aria-hidden="true" />
                            </Button>
                          </div>
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((row) => (
                      <tr key={row.label} className="align-top">
                        <th scope="row" className="sticky left-0 z-10 border-r border-t border-[var(--border)] bg-[var(--surface-raised)] px-4 py-3 text-left text-xs font-medium muted">{row.label}</th>
                        {data.models.map((model) => (
                          <td key={model.id} className="border-r border-t border-[var(--border)] px-4 py-3 leading-6 last:border-r-0">{row.render(model)}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <section aria-labelledby="comparison-trends-title">
                <div className="mb-3">
                  <h3 id="comparison-trends-title" className="m-0 text-base font-semibold">性能趋势</h3>
                  <p className="mt-1 mb-0 text-xs muted">{window === "7d" ? "近 7 天 · 每日汇总" : "近 24 小时 · 每小时汇总"}</p>
                </div>
                <div className="grid gap-4 xl:grid-cols-2">
                  {(["tpm", "rpm", "ttft", "tpot"] as const).map((key) => (
                    <div key={key} className="min-w-0 rounded-[10px] border border-[var(--border)] p-3">
                      <h4 className="m-0 text-sm font-semibold">{key.toUpperCase()} 趋势</h4>
                      <Chart label={key.toUpperCase() + " 对比趋势"} height={250} option={buildComparisonTrendOption(data, key)} />
                    </div>
                  ))}
                </div>
              </section>
            </div>
          ) : (
            <div className="py-16 text-center text-sm muted">请选择至少两个模型。</div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
