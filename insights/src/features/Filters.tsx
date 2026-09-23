import type {
  DepartmentOption,
  FilterState,
  Granularity,
  ModelOption,
} from "../lib/types";
import { Input, Select } from "../components/ui/Controls";
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
  const set = <K extends keyof FilterState>(k: K, v: FilterState[K]) =>
    onChange({ ...value, [k]: v });
  return (
    <div className="flex flex-wrap items-end gap-3">
      <label className="grid gap-1 text-xs muted">
        开始日期
        <Input
          type="date"
          value={value.from}
          max={value.to}
          onChange={(e) => set("from", e.target.value)}
        />
      </label>
      <label className="grid gap-1 text-xs muted">
        结束日期
        <Input
          type="date"
          value={value.to}
          min={value.from}
          onChange={(e) => set("to", e.target.value)}
        />
      </label>
      <label className="grid gap-1 text-xs muted">
        粒度
        <Select
          value={value.granularity}
          onChange={(e) => set("granularity", e.target.value as Granularity)}
        >
          {granularities.map((g) => (
            <option key={g} value={g}>
              {
                ({ hour: "小时", day: "天", week: "周", month: "月" } as const)[
                  g
                ]
              }
            </option>
          ))}
        </Select>
      </label>
      {showModels && (
        <label className="grid min-w-48 gap-1 text-xs muted">
          模型（可多选）
          <select
            multiple
            aria-label="模型筛选"
            value={value.models}
            onChange={(e) =>
              set(
                "models",
                [...e.target.selectedOptions].map((o) => o.value),
              )
            }
            className="min-h-20 rounded-md border border-[var(--border)] bg-[var(--surface)] p-2 text-sm text-[var(--ink)]"
          >
            {models.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name} · {m.platform}
              </option>
            ))}
          </select>
        </label>
      )}
      {showDepartments && (
        <label className="grid min-w-48 gap-1 text-xs muted">
          部门（可多选）
          <select
            multiple
            aria-label="部门筛选"
            value={value.departments}
            onChange={(e) =>
              set(
                "departments",
                [...e.target.selectedOptions].map((o) => o.value),
              )
            }
            className="min-h-20 rounded-md border border-[var(--border)] bg-[var(--surface)] p-2 text-sm text-[var(--ink)]"
          >
            {departments?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
                {d.issue ? "（配置异常）" : ""}
              </option>
            ))}
          </select>
        </label>
      )}
    </div>
  );
}
