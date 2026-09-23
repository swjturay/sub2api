import { useEffect, useRef, useState } from "react";
import { ExternalLink } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/Dialog";
import type { Capability, ModelProfile } from "../../lib/types";
import { PricingList } from "./PricingList";
import { restoreModelDialogFocus } from "./modelDialogFocus";

const capability = (value: Capability) => value === "supported" ? "支持" : value === "unsupported" ? "不支持" : "未知";
function safeUrl(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
}

export function ModelDetailDialog({ model, close, restoreFocusElement }: {
  model: ModelProfile;
  close: () => void;
  restoreFocusElement: HTMLElement | null;
}) {
  const [open, setOpen] = useState(true);
  const closeTimer = useRef<number | null>(null);

  useEffect(() => () => {
    if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
  }, []);

  const changeOpen = (nextOpen: boolean) => {
    setOpen(nextOpen);
    if (!nextOpen) {
      if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
      closeTimer.current = window.setTimeout(close, 0);
    }
  };

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent
        className="w-[min(calc(100vw-24px),64rem)] max-w-5xl"
        onCloseAutoFocus={(event) => restoreModelDialogFocus(event, restoreFocusElement)}
      >
        <DialogHeader>
          <DialogTitle>{model.name}</DialogTitle>
          <DialogDescription>{model.platform} · 官方模型资料与当前聚合指标</DialogDescription>
        </DialogHeader>
        <div className="mt-5"><ProfileView model={model} /></div>
      </DialogContent>
    </Dialog>
  );
}

function ProfileView({ model }: { model: ModelProfile }) {
  const facts = [
    ["上下文", model.contextLimit?.toLocaleString("zh-CN") || "未知"],
    ["最大输出", model.maxOutput?.toLocaleString("zh-CN") || "未知"],
    ["输入模态", model.inputModalities.join("、") || "未知"],
    ["输出模态", model.outputModalities.join("、") || "未知"],
  ];
  return (
    <div className="grid gap-6">
      <section>
        <h3 className="m-0 text-sm font-semibold">模型介绍</h3>
        <p className="mt-2 mb-0 max-w-3xl text-sm leading-6 muted">{model.description || "暂无介绍。"}</p>
        {model.useCases.length > 0 && <div className="mt-3 flex flex-wrap gap-2">{model.useCases.map((item) => <span key={item} className="rounded-full bg-[var(--primary-soft)] px-2.5 py-1 text-xs text-[var(--primary)]">{item}</span>)}</div>}
      </section>
      <dl className="grid gap-px overflow-hidden rounded-[10px] border border-[var(--border)] bg-[var(--border)] sm:grid-cols-2 lg:grid-cols-4">
        {facts.map(([label, value]) => (
          <div key={label} className="bg-[var(--surface)] p-3">
            <dt className="text-xs muted">{label}</dt>
            <dd className="m-0 mt-1 break-words text-sm font-semibold tabular-nums">{value}</dd>
          </div>
        ))}
      </dl>
      <section>
        <h3 className="m-0 text-sm font-semibold">能力</h3>
        <div className="mt-2 flex flex-wrap gap-2 text-xs">
          <span className="rounded-full border border-[var(--border)] px-2.5 py-1">推理：{capability(model.reasoning)}</span>
          <span className="rounded-full border border-[var(--border)] px-2.5 py-1">工具调用：{capability(model.toolCalling)}</span>
          <span className="rounded-full border border-[var(--border)] px-2.5 py-1">结构化输出：{capability(model.structuredOutput)}</span>
        </div>
      </section>
      <section><h3 className="m-0 text-sm font-semibold">参考价格</h3><div className="mt-2"><PricingList pricing={model.pricing} /></div></section>
      <section>
        <h3 className="m-0 text-sm font-semibold">资料来源</h3>
        {model.sources.length ? (
          <ul className="mt-2 grid gap-2 pl-5 text-sm">
            {model.sources.map((source, index) => {
              const href = safeUrl(source.url);
              return <li key={source.url + "-" + index}>{href ? <a className="inline-flex items-center gap-1 text-[var(--primary)] underline-offset-2 hover:underline" href={href} target="_blank" rel="noreferrer">{source.label || source.url}<ExternalLink className="h-3.5 w-3.5" aria-hidden="true" /></a> : source.label || source.url}{source.updatedAt && <span className="muted"> · {source.updatedAt.slice(0, 10)}</span>}</li>;
            })}
          </ul>
        ) : <p className="mt-2 mb-0 text-sm muted">未配置</p>}
      </section>
    </div>
  );
}
