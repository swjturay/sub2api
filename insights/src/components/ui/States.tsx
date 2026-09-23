import { AlertCircle, AlertTriangle, Database, Info } from "lucide-react";
import { Button } from "./Button";
import type { Coverage } from "../../lib/types";
export function Loading({ label = "正在读取数据" }: { label?: string }) {
  return (
    <div className="loading-skeleton panel" role="status" aria-label={label}>
      <span className="sr-only">{label}</span>
      <div className="skeleton-line w-28" />
      <div className="grid grid-cols-3 gap-3"><div className="skeleton-block" /><div className="skeleton-block" /><div className="skeleton-block" /></div>
      <div className="skeleton-chart" />
    </div>
  );
}
export function Empty({
  title = "暂无数据",
  detail = "当前筛选范围没有可展示的记录。",
}: {
  title?: string;
  detail?: string;
}) {
  return (
    <div className="empty-state">
      <Database className="h-7 w-7 muted" aria-hidden="true" />
      <strong>{title}</strong>
      <p className="m-0 max-w-lg text-sm muted">{detail}</p>
    </div>
  );
}
export function ErrorBanner({
  message,
  retry,
  stale,
}: {
  message: string;
  retry: () => void;
  stale?: boolean;
}) {
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center gap-3 rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200"
    >
      <AlertTriangle className="h-4 w-4" aria-hidden="true" />
      <span className="flex-1">
        {stale ? "刷新失败，继续显示上次成功数据：" : "加载失败："}
        {message}
      </span>
      <Button variant="secondary" onClick={retry}>
        重试
      </Button>
    </div>
  );
}
export function CoverageBanner({ coverage }: { coverage: Coverage }) {
  if (coverage.state === "complete") return null;
  const title = coverage.state === "partial" ? "部分覆盖" : coverage.state === "uncollected" ? "尚未采集" : "覆盖未知";
  const Icon = coverage.state === "unavailable" ? AlertCircle : Info;
  return (
    <div className="coverage-note" data-coverage={coverage.state} role="note">
      <Icon aria-hidden="true" />
      <span><strong>{title}</strong>{coverage.message && ` · ${coverage.message}`}</span>
    </div>
  );
}
