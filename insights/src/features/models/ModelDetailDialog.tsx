import { useEffect, useRef, useState, type ReactNode } from "react";
import { ExternalLink, SlidersHorizontal } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { Input, Select } from "../../components/ui/Controls";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/Dialog";
import { insightsApi } from "../../lib/api";
import { ApiRequestError } from "../../lib/auth";
import type { Capability, ModelProfile } from "../../lib/types";
import { PricingList } from "./PricingList";
import { restoreModelDialogFocus } from "./modelDialogFocus";

const capability = (value: Capability) => value === "supported" ? "支持" : value === "unsupported" ? "不支持" : "未知";
const splitLines = (value: string) => value.split(String.fromCharCode(10)).map((item) => item.replace(String.fromCharCode(13), ""));

function safeUrl(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
}

export function ModelDetailDialog({ model, admin, close, reload, onSaved, restoreFocusElement }: {
  model: ModelProfile;
  admin: boolean;
  close: () => void;
  reload: () => Promise<void>;
  onSaved: () => Promise<void>;
  restoreFocusElement: HTMLElement | null;
}) {
  const [open, setOpen] = useState(true);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(model);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const closeTimer = useRef<number | null>(null);
  const canEdit = admin && model.editable;

  useEffect(() => {
    if (!editing) setDraft(model);
  }, [editing, model]);
  useEffect(() => () => {
    if (closeTimer.current !== null) window.clearTimeout(closeTimer.current);
  }, []);

  const set = <K extends keyof ModelProfile>(key: K, value: ModelProfile[K]) => setDraft((current) => ({ ...current, [key]: value }));
  const cancelEditing = () => {
    setDraft(model);
    setEditing(false);
    setError("");
    setConflict(false);
  };
  const save = async () => {
    setSaving(true);
    setError("");
    setConflict(false);
    try {
      await insightsApi.saveModel(model.id, draft);
      await onSaved();
      setEditing(false);
    } catch (caught) {
      if (caught instanceof ApiRequestError && caught.status === 409) {
        setConflict(true);
        setError("资料已被其他管理员更新，请重新加载后再编辑。");
      } else {
        setError((caught as Error).message);
      }
    } finally {
      setSaving(false);
    }
  };
  const reloadLatest = async () => {
    setEditing(false);
    setError("");
    setConflict(false);
    await reload();
  };
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
          <DialogDescription>{model.platform} · 模型资料与当前聚合指标</DialogDescription>
        </DialogHeader>
        {error && (
          <div role="alert" className="mt-4 flex flex-wrap items-center gap-2 rounded-[8px] border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200">
            <span className="flex-1">{error}</span>
            {conflict && <Button variant="secondary" onClick={() => void reloadLatest()}>重新加载</Button>}
          </div>
        )}
        <div className="mt-5">{editing ? <ProfileForm value={draft} set={set} /> : <ProfileView model={model} />}</div>
        <DialogFooter>
          {canEdit && (editing ? (
            <>
              <Button variant="secondary" disabled={saving} onClick={cancelEditing}>取消</Button>
              <Button disabled={saving} onClick={() => void save()}>{saving ? "保存中" : "保存资料"}</Button>
            </>
          ) : <Button onClick={() => setEditing(true)}><SlidersHorizontal aria-hidden="true" />编辑资料</Button>)}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ProfileForm({ value, set }: {
  value: ModelProfile;
  set: <K extends keyof ModelProfile>(key: K, value: ModelProfile[K]) => void;
}) {
  const capabilities: Array<[keyof Pick<ModelProfile, "reasoning" | "toolCalling" | "structuredOutput">, string]> = [
    ["reasoning", "推理"],
    ["toolCalling", "工具调用"],
    ["structuredOutput", "结构化输出"],
  ];
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Field label="介绍" wide><textarea className="min-h-24 rounded-[10px] border border-[var(--border)] bg-[var(--surface)] p-3 text-sm" value={value.description || ""} onChange={(event) => set("description", event.target.value || null)} /></Field>
      <Field label="适用场景（每行一项）" wide><textarea className="min-h-24 rounded-[10px] border border-[var(--border)] bg-[var(--surface)] p-3 text-sm" value={value.useCases.join(String.fromCharCode(10))} onChange={(event) => set("useCases", splitLines(event.target.value).map((item) => item.trim()).filter(Boolean))} /></Field>
      <NumberField label="上下文上限" value={value.contextLimit} set={(next) => set("contextLimit", next)} />
      <NumberField label="最大输出" value={value.maxOutput} set={(next) => set("maxOutput", next)} />
      <TextList label="输入模态（逗号分隔）" value={value.inputModalities} set={(next) => set("inputModalities", next)} />
      <TextList label="输出模态（逗号分隔）" value={value.outputModalities} set={(next) => set("outputModalities", next)} />
      {capabilities.map(([key, label]) => (
        <Field label={label} key={key}><Select value={value[key]} onChange={(event) => set(key, event.target.value as Capability)}><option value="unknown">未知</option><option value="supported">支持</option><option value="unsupported">不支持</option></Select></Field>
      ))}
      <Field label="资料来源（每行：标签 | URL | YYYY-MM-DD）" wide>
        <textarea
          className="min-h-28 rounded-[10px] border border-[var(--border)] bg-[var(--surface)] p-3 text-sm"
          value={value.sources.map((source) => [source.label, source.url, source.updatedAt?.slice(0, 10) || ""].join(" | ")).join(String.fromCharCode(10))}
          onChange={(event) => set("sources", splitLines(event.target.value).map((line) => {
            const [label = "", url = "", updatedAt = ""] = line.split("|").map((part) => part.trim());
            return { label, url, updatedAt: updatedAt || null };
          }).filter((source) => source.label || source.url))}
        />
      </Field>
    </div>
  );
}

function Field({ label, children, wide = false }: { label: string; children: ReactNode; wide?: boolean }) {
  return <label className={(wide ? "sm:col-span-2 " : "") + "grid gap-1.5 text-sm font-medium"}>{label}{children}</label>;
}

function NumberField({ label, value, set }: { label: string; value: number | null; set: (value: number | null) => void }) {
  return <Field label={label}><Input type="number" min="0" step="1" value={value ?? ""} onChange={(event) => set(event.target.value ? Number(event.target.value) : null)} /></Field>;
}

function TextList({ label, value, set }: { label: string; value: string[]; set: (value: string[]) => void }) {
  return <Field label={label}><Input value={value.join(", ")} onChange={(event) => set(event.target.value.split(",").map((item) => item.trim()).filter(Boolean))} /></Field>;
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
