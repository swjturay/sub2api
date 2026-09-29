import { useEffect, useRef, useState } from "react";
import { Segmented } from "../../components/ui/Controls";
import { ErrorBanner } from "../../components/ui/States";
import { insightsApi } from "../../lib/api";
import { metric } from "../../lib/format";
import type { ModelProfile } from "../../lib/types";

export function ModelPerformance({ model }: { model: ModelProfile }) {
  const [window, setWindow] = useState<"24h" | "7d">("24h");
  const [metrics, setMetrics] = useState<ModelProfile["metrics"] | null>(model.metrics);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);

  const changeWindow = async (next: "24h" | "7d") => {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setWindow(next);
    setMetrics(null);
    setError(null);
    setLoading(true);
    try {
      const result = await insightsApi.model(model.id, next, controller.signal);
      if (!controller.signal.aborted) setMetrics(result.metrics);
    } catch (caught) {
      if (!controller.signal.aborted) setError((caught as Error).message);
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  };

  return <section aria-label="模型实测性能">
    <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 className="m-0 text-sm font-semibold">实测性能</h3>
      <Segmented value={window} onChange={next => void changeWindow(next)} label="性能窗口" options={[{ value: "24h", label: "近 24 小时" }, { value: "7d", label: "近 7 天" }]} />
    </div>
    {error && <div className="mb-3"><ErrorBanner message={error} retry={() => void changeWindow(window)} /></div>}
    {loading && <span className="sr-only" role="status">正在加载{window === "7d" ? "近 7 天" : "近 24 小时"}性能</span>}
    <dl className="m-0 grid grid-cols-2 gap-3" aria-busy={loading}>
      {([
        ["平均 TPM", "tpm", "", "每分钟 Token 数"],
        ["平均 RPM", "rpm", "", "每分钟请求数"],
        ["平均 TTFT", "ttft", "ms", "首字延迟"],
        ["估算 TPOT", "tpot", "ms", "估算每 Token 耗时"],
      ] as const).map(([label, key, unit, description]) => <div className="rounded-lg border border-[var(--border)] p-3" key={key}>
        <dt className="text-xs muted" title={description}>{label}</dt>
        <dd className="m-0 mt-2 text-lg font-semibold tabular-nums">{loading ? "…" : metric(metrics?.[key].value ?? null, unit)}</dd>
        {metrics?.[key].sampleCount !== undefined && <p className="mb-0 mt-1 text-xs muted">{metrics[key].sampleCount} 个有效样本</p>}
      </div>)}
    </dl>
  </section>;
}
