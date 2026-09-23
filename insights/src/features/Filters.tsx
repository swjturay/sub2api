import { useState } from "react";
import { CalendarDays, ChevronDown } from "lucide-react";
import { zhCN } from "react-day-picker/locale";
import type { DateRange } from "react-day-picker";
import type { DepartmentOption, FilterState, Granularity, ModelOption } from "../lib/types";
import { Button } from "../components/ui/Button";
import { Calendar } from "../components/ui/Calendar";
import { Input, Segmented } from "../components/ui/Controls";
import { MultiSelect } from "../components/ui/MultiSelect";
import { Popover, PopoverContent, PopoverTrigger } from "../components/ui/Popover";
import { useBootstrap } from "./BootstrapContext";
import { platformDateParts } from "../lib/format";

const granularityLabels: Record<Granularity, string> = { hour: "小时", day: "天", week: "周", month: "月" };
const dayPattern = /^(\d{4})-(\d{2})-(\d{2})$/;

function parseDay(value: string) {
  const match = dayPattern.exec(value);
  if (!match) return undefined;
  const [, year, month, day] = match;
  const parsed = new Date(Number(year), Number(month) - 1, Number(day), 12);
  if (parsed.getFullYear() !== Number(year) || parsed.getMonth() !== Number(month) - 1 || parsed.getDate() !== Number(day)) return undefined;
  return parsed;
}

function formatDay(value: Date) {
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`;
}

function subtractDays(day: string, count: number) {
  const match = dayPattern.exec(day);
  if (!match) return day;
  const value = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3]) - count));
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${value.getUTCFullYear()}-${pad(value.getUTCMonth() + 1)}-${pad(value.getUTCDate())}`;
}

function DateRangeFilter({ from, to, today, onChange }: { from: string; to: string; today: string; onChange: (from: string, to: string) => void }) {
  const [open, setOpen] = useState(false);
  const [draftFrom, setDraftFrom] = useState(from);
  const [draftTo, setDraftTo] = useState(to);
  const start = parseDay(draftFrom), end = parseDay(draftTo);
  const valid = Boolean(start && end && draftFrom <= draftTo);
  const selected: DateRange | undefined = start ? { from: start, to: end } : undefined;
  const updateOpen = (next: boolean) => {
    setOpen(next);
    if (next) { setDraftFrom(from); setDraftTo(to); }
  };
  const apply = () => {
    if (!valid) return;
    onChange(draftFrom, draftTo);
    setOpen(false);
  };
  const preset = (days: number) => {
    onChange(subtractDays(today, days - 1), today);
    setOpen(false);
  };
  return (
    <Popover open={open} onOpenChange={updateOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="secondary" className="filter-trigger min-w-60 justify-start font-medium" aria-label="日期范围">
          <CalendarDays aria-hidden="true" />
          <span className="tabular-nums">{from} – {to}</span>
          <ChevronDown className="ml-auto text-[var(--muted)]" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="date-range-popover w-[36rem] max-w-[calc(100vw-32px)] p-0" align="start">
        <div className="flex border-b border-[var(--border)] p-3">
          {[{ label: "今天", days: 1 }, { label: "近 7 天", days: 7 }, { label: "近 30 天", days: 30 }].map((item) => <Button key={item.days} type="button" variant="ghost" onClick={() => preset(item.days)}>{item.label}</Button>)}
        </div>
        <div className="grid grid-cols-[1fr_14rem]">
          <Calendar mode="range" locale={zhCN} selected={selected} onSelect={(range) => { setDraftFrom(range?.from ? formatDay(range.from) : ""); setDraftTo(range?.to ? formatDay(range.to) : ""); }} defaultMonth={start || end} numberOfMonths={1} />
          <div className="border-l border-[var(--border)] p-4">
            <div className="grid gap-3">
              <label className="field-label">开始日期<Input value={draftFrom} inputMode="numeric" placeholder="YYYY-MM-DD" aria-invalid={Boolean(draftFrom) && !start} onChange={(event) => setDraftFrom(event.target.value)} /></label>
              <label className="field-label">结束日期<Input value={draftTo} inputMode="numeric" placeholder="YYYY-MM-DD" aria-invalid={Boolean(draftTo) && !end} onChange={(event) => setDraftTo(event.target.value)} /></label>
            </div>
            {!valid && <p className="mb-0 mt-3 text-xs text-[var(--danger)]" role="status">请输入完整且有效的日期范围。</p>}
            <div className="mt-5 flex justify-end gap-2"><Button type="button" variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button type="button" disabled={!valid} onClick={apply}>应用</Button></div>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function AnalyticsFilters({
  value,
  onChange,
  models,
  departments,
  granularities = ["day", "week", "month"],
  showModels = true,
  showDepartments = false,
}: {
  value: FilterState;
  onChange: (v: FilterState) => void;
  models: ModelOption[];
  departments?: DepartmentOption[];
  granularities?: Granularity[];
  showModels?: boolean;
  showDepartments?: boolean;
}) {
  const { timezone } = useBootstrap();
  const platformToday = platformDateParts(new Date().toISOString(), timezone).date;
  const set = <K extends keyof FilterState>(key: K, next: FilterState[K]) => onChange({ ...value, [key]: next });
  return (
    <div className="analytics-filters" aria-label="分析筛选条件">
      <div className="filter-field filter-field--range"><span className="filter-label sr-only">日期范围</span><DateRangeFilter from={value.from} to={value.to} today={platformToday} onChange={(from, to) => onChange({ ...value, from, to })} /></div>
      <div className="filter-field"><span className="filter-label sr-only">粒度</span><Segmented value={value.granularity} onChange={(next) => set("granularity", next)} label="统计粒度" options={granularities.map((granularity) => ({ value: granularity, label: granularityLabels[granularity] }))} /></div>
      {showModels && <div className="filter-field"><span className="filter-label sr-only">模型</span><MultiSelect label="模型筛选" value={value.models} onChange={(next) => set("models", next)} options={models.map((model) => ({ value: model.id, label: model.name, keywords: model.platform }))} allLabel="全部模型" selectedLabel="个模型" searchPlaceholder="搜索模型或平台" emptyText="没有匹配模型" /></div>}
      {showDepartments && <div className="filter-field"><span className="filter-label sr-only">部门</span><MultiSelect label="部门筛选" value={value.departments} onChange={(next) => set("departments", next)} options={(departments || []).map((department) => ({ value: department.id, label: department.issue ? `${department.name}（配置异常）` : department.name, keywords: department.issue }))} allLabel="全部部门" selectedLabel="个部门" searchPlaceholder="搜索部门" emptyText="没有匹配部门" /></div>}
    </div>
  );
}
