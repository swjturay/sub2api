import { AlertTriangle, Database, LoaderCircle } from "lucide-react";
import { Button } from "./Button";
import type { Coverage } from "../../lib/types";
export function Loading({ label = "正在读取数据" }: { label?: string }) {
  return (
    <div className="panel flex min-h-48 items-center justify-center gap-2 muted">
      <LoaderCircle className="h-5 w-5 animate-spin" />
      <span>{label}</span>
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
    <div className="flex min-h-44 flex-col items-center justify-center gap-2 text-center">
      <Database className="h-7 w-7 muted" />
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
      <AlertTriangle className="h-4 w-4" />
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
  return (
    <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
      <strong>
        {coverage.state === "partial"
          ? "数据不完整"
          : coverage.state === "uncollected"
            ? "尚未采集"
            : "数据不可用"}
      </strong>
      {coverage.message && `：${coverage.message}`}
    </div>
  );
}
