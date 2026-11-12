/* eslint-disable react-refresh/only-export-components */
import { useMemo, useState } from "react";
import { ChevronDown } from "lucide-react";
import type { EChartsOption } from "echarts";
import type { CostFilters, DepartmentAnalytics, DepartmentMemberCount, TimePoint } from "../lib/types";
import { Chart, palette } from "../components/charts/Chart";
import { cartesianTheme, chartColors, compactNumber, fullNumber, legendTheme, tooltipTheme } from "../components/charts/chartTheme";
import { HelpTip } from "../components/ui/HelpTip";
import { Segmented } from "../components/ui/Controls";
import { Empty, ErrorBanner, Loading } from "../components/ui/States";
import { useBootstrap } from "../features/BootstrapContext";
import { insightsApi } from "../lib/api";
import { percent, platformDateParts } from "../lib/format";
import { useRemote } from "../lib/useRemote";

type ModelChartMode = "pie" | "bar";
type CostCurrency = "usd" | "cny";
type HeavyUser = { id: string | number; name: string; tokens: number };
const CNY_RATE = 6.7;

const familyLabels: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  deepseek: "DeepSeek",
  zai: "ZhipuGLM",
  zhipu: "ZhipuGLM",
  glm: "ZhipuGLM",
  gemini: "Gemini",
  antigravity: "Gemini",
  xai: "xAI",
  grok: "xAI",
  opencode: "其他",
  opencode_go: "其他",
  opencodego: "其他",
  qwen: "Qwen",
  unknown: "其他",
};

export function monthKey(value: string) {
  return /^(\d{4})-(\d{2})/.exec(value)?.slice(1).join("-") || value;
}

export function monthLabel(value: string) {
  const match = /^(\d{4})-(\d{2})/.exec(value);
  return match ? `${match[1]}年${match[2]}月` : value;
}

function familyName(modelId: string, modelName: string, platform: string) {
  const identity = `${modelId} ${modelName}`.toLowerCase();
  if (identity.includes("deepseek")) return "DeepSeek";
  if (identity.includes("glm")) return "ZhipuGLM";
  if (identity.includes("claude")) return "Anthropic";
  if (identity.includes("gemini")) return "Gemini";
  if (identity.includes("grok")) return "xAI";
  if (identity.includes("qwen")) return "Qwen";
  if (identity.includes("gpt") || identity.includes("codex") || /(^|[:/_-])o[134]([:/_-]|$)/.test(identity)) return "OpenAI";
  const prefix = (platform || modelId.split(":", 1)[0] || "unknown").toLowerCase();
  return familyLabels[prefix] || prefix.replace(/(^|[-_])(\w)/g, (_, __, letter: string) => letter.toUpperCase());
}

export function modelFamilyData(series: TimePoint[], month: string) {
  const totals = new Map<string, number>();
  for (const point of series) {
    if (month !== "all" && monthKey(point.bucket) !== month) continue;
    for (const model of point.models || []) {
      const family = familyName(model.modelId, model.name, model.platform);
      totals.set(family, (totals.get(family) || 0) + (model.totalTokens || 0));
    }
  }
  return [...totals].map(([name, value]) => ({ name, value })).filter((item) => item.value > 0).sort((a, b) => b.value - a.value);
}

export function modelFamilyMonthlyData(series: TimePoint[]) {
  const months = [...new Set(series.map((point) => monthKey(point.bucket)))];
  const values = new Map<string, Map<string, number>>();
  for (const point of series) {
    const month = monthKey(point.bucket);
    for (const model of point.models || []) {
      const family = familyName(model.modelId, model.name, model.platform);
      const familyValues = values.get(family) || new Map<string, number>();
      familyValues.set(month, (familyValues.get(month) || 0) + (model.totalTokens || 0));
      values.set(family, familyValues);
    }
  }
  const families = [...values].map(([name, familyValues]) => ({
    name,
    total: [...familyValues.values()].reduce((sum, value) => sum + value, 0),
    values: months.map((month) => familyValues.get(month) || 0),
  })).filter((family) => family.total > 0).sort((a, b) => b.total - a.total);
  return { months, families };
}

export function heavyUsersForMonth(data: DepartmentAnalytics, month: string): HeavyUser[] {
  if (month === "all") return [...data.memberPareto].sort((a, b) => b.tokens - a.tokens).slice(0, 10).map((user) => ({ id: user.id, name: user.name, tokens: user.tokens }));
  const members = new Map<number, HeavyUser>();
  for (const point of data.series) {
    if (monthKey(point.bucket) !== month) continue;
    for (const user of point.members || []) {
      const current = members.get(user.id) || { id: user.id, name: user.name, tokens: 0 };
      current.tokens += user.totalTokens;
      members.set(user.id, current);
    }
  }
  return [...members.values()].sort((a, b) => b.tokens - a.tokens).slice(0, 10);
}

function NativeMonthSelect({ label, value, months, onChange }: { label: string; value: string; months: string[]; onChange: (value: string) => void }) {
  return <span className="relative inline-flex">
    <select aria-label={label} className="h-9 min-w-36 appearance-none rounded-md border border-[var(--border)] bg-[var(--surface)] py-1 pl-3 pr-9 text-sm text-[var(--ink)] shadow-[var(--shadow-sm)] outline-none hover:border-[var(--border-strong)] focus-visible:ring-2 focus-visible:ring-[var(--ring)]" value={value} onChange={(event) => onChange(event.target.value)}>
      <option value="all">全部月份</option>
      {months.map((month) => <option value={month} key={month}>{monthLabel(month)}</option>)}
    </select>
    <ChevronDown aria-hidden="true" className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-[var(--muted)]" />
  </span>;
}

export function UserGrowthCard({ series, loading, error, retry }: { series?: TimePoint[]; loading: boolean; error?: string; retry: () => void }) {
  const data = series || [];
  const theme = cartesianTheme(54);
  const colors = chartColors();
  return <section className="panel h-full">
    <h2 className="m-0 flex items-center gap-2 text-base font-semibold">用户数<HelpTip label="用户数说明">全平台注册用户截至各月末的累计数量，仅跟随日期范围，用于观察平台用户增长。</HelpTip></h2>
    {loading && !series ? <Loading label="正在加载用户数" /> : error && !series ? <ErrorBanner message={error} retry={retry} /> : data.length ? <Chart label="平台月末总用户增长折线图" height={280} option={{
      color: [palette[0]], textStyle: theme.textStyle,
      tooltip: { ...tooltipTheme(), trigger: "axis", valueFormatter: (value) => `${fullNumber(value)} 人` },
      grid: { ...(theme.grid as object), top: 40, bottom: 42 },
      xAxis: { ...theme.xAxis, type: "category", boundaryGap: false, data: data.map((point) => monthLabel(point.bucket)), axisLabel: { ...(theme.xAxis.axisLabel as object), hideOverlap: true } },
      yAxis: { ...theme.yAxis, type: "value", min: 0, axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: (value: unknown) => `${compactNumber(value)}人` } },
      series: [{ name: "总用户数", type: "line", smooth: 0.22, symbolSize: 7, showSymbol: true, data: data.map((point) => point.users ?? null), lineStyle: { width: 2.5 }, areaStyle: { color: palette[0], opacity: 0.12 }, label: { show: true, position: "top", color: colors.ink, fontSize: 12, formatter: (params: { value?: unknown }) => compactNumber(params.value) } }],
    }} /> : <Empty title="暂无用户增长数据" detail="当前日期范围没有可展示的月末用户数据。" />}
  </section>;
}

export function groupedDepartments(input: DepartmentMemberCount[]) {
  const unassigned = input.filter((item) => item.id === "__unassigned__" || item.name.trim() === "未分配");
  const sorted = input.filter((item) => !unassigned.includes(item)).sort((a, b) => b.count - a.count || a.id.localeCompare(b.id));
  const visible = sorted.slice(0, 7).map((item) => ({ name: item.name, value: item.count }));
  const hidden = sorted.slice(7);
  const groupedValue = [...hidden, ...unassigned].reduce((sum, item) => sum + item.count, 0);
  if (groupedValue <= 0) return visible;
  const groupedCount = hidden.length + unassigned.length;
  return [...visible, { name: groupedCount > 1 ? `其他 ${groupedCount} 个部门` : "其他", value: groupedValue }];
}

export function pastCostTrend<T extends { month: string }>(trend: T[], currentMonth: string) {
  return trend.filter((point) => point.month < currentMonth);
}

export function cumulativeActualCost(trend: Array<{ actualCost: number }>) {
  return trend.reduce((sum, point) => sum + point.actualCost, 0);
}

export function convertCost(value: number, currency: CostCurrency) {
  return currency === "cny" ? value * CNY_RATE : value;
}

export function UserDepartmentDistributionCard({ distribution }: { distribution: DepartmentMemberCount[] }) {
  const data = groupedDepartments(distribution);
  const total = data.reduce((sum, item) => sum + item.value, 0);
  const colors = chartColors();
  return <section className="panel h-full">
    <h2 className="m-0 flex items-center gap-2 text-base font-semibold">用户部门分布<HelpTip label="用户部门分布说明">全站未删除用户按当前部门统计，不跟随上方部门或模型筛选。</HelpTip></h2>
    {data.length ? <Chart label="所有用户部门分布饼状图" height={280} option={{
      color: palette,
      tooltip: { ...tooltipTheme(), trigger: "item", formatter: (parameter: unknown) => {
        const item = parameter as { name: string; value: number };
        return `${item.name}<br/><b>${fullNumber(item.value)}</b> 人 · ${percent(total ? item.value / total : null)}`;
      } },
      legend: { ...legendTheme(), type: "scroll", bottom: 0 },
      series: [{ type: "pie", radius: ["38%", "66%"], center: ["50%", "43%"], avoidLabelOverlap: true, label: { color: colors.muted, formatter: "{b}\n{c}人" }, labelLine: { length: 10, length2: 8 }, itemStyle: { borderColor: colors.surface, borderWidth: 2, borderRadius: 4 }, data }],
    }} /> : <Empty title="暂无部门分布数据" detail="当前尚未配置可展示的用户部门数据。" />}
  </section>;
}

export function ModelFamilyDistributionCard({ series }: { series: TimePoint[] }) {
  const months = useMemo(() => [...new Set(series.map((point) => monthKey(point.bucket)))], [series]);
  const [month, setMonth] = useState("all");
  const [mode, setMode] = useState<ModelChartMode>("pie");
  const data = modelFamilyData(series, month);
  const monthly = modelFamilyMonthlyData(series);
  const total = data.reduce((sum, item) => sum + item.value, 0);
  const theme = cartesianTheme(62);
  const colors = chartColors();
  const option: EChartsOption = mode === "pie" ? {
    color: palette,
    tooltip: { ...tooltipTheme(), trigger: "item", formatter: (parameter: unknown) => {
      const item = parameter as { name: string; value: number };
      return `${item.name}<br/><b>${fullNumber(item.value)}</b> Token · ${percent(total ? item.value / total : null)}`;
    } },
    legend: { ...legendTheme(), type: "scroll", bottom: 0 },
    series: [{ type: "pie", radius: ["42%", "70%"], center: ["50%", "43%"], label: { color: colors.muted, formatter: "{b}\n{d}%" }, itemStyle: { borderColor: colors.surface, borderWidth: 2, borderRadius: 4 }, data }],
  } : {
    color: palette, textStyle: theme.textStyle,
    tooltip: { ...tooltipTheme(), trigger: "axis", axisPointer: { type: "shadow" }, valueFormatter: (value: unknown) => `${fullNumber(value)} Token` },
    legend: { ...legendTheme(), type: "scroll", bottom: 0 },
    grid: { ...(theme.grid as object), top: 32, bottom: 74 },
    xAxis: { ...theme.xAxis, type: "category", data: monthly.months.map(monthLabel), axisLabel: { ...(theme.xAxis.axisLabel as object), hideOverlap: true } },
    yAxis: { ...theme.yAxis, type: "value", axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: compactNumber } },
    series: monthly.families.map((family, index) => ({ name: family.name, type: "bar", stack: "tokens", barMaxWidth: 48, data: family.values, emphasis: { focus: "series" }, itemStyle: { borderRadius: index === monthly.families.length - 1 ? [4, 4, 0, 0] : 0 } })),
  };
  return <section className="panel h-full">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 className="m-0 flex items-center gap-2 text-base font-semibold">模型使用比例<HelpTip label="模型使用比例说明">按实际使用的模型名称归入 OpenAI、DeepSeek、ZhipuGLM 等系列；饼状图可查看全部或指定月份，柱状图按月份堆叠展示。</HelpTip></h2>
      <div className="flex flex-wrap items-center gap-2">
        {mode === "pie" && <NativeMonthSelect label="模型系列分布月份" value={month} months={months} onChange={setMonth} />}
        <Segmented value={mode} onChange={setMode} label="模型系列图表类型" options={[{ value: "pie", label: "饼状" }, { value: "bar", label: "柱状" }]} />
      </div>
    </div>
    {data.length || mode === "bar" ? <Chart label={`模型系列总Token${mode === "pie" ? "饼状图" : "月度堆叠柱状图"}`} height={280} option={option} /> : <Empty title="暂无模型系列数据" />}
  </section>;
}

export function HeavyUsersCard({ data }: { data: DepartmentAnalytics }) {
  const months = [...new Set(data.series.map((point) => monthKey(point.bucket)))];
  const [month, setMonth] = useState("all");
  const users = heavyUsersForMonth(data, month);
  const maximum = users[0]?.tokens || 1;
  return <section className="panel h-[340px]">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 className="m-0 flex items-center gap-2 text-base font-semibold">重度用户 TOP10<HelpTip label="重度用户TOP10说明">按总 Token 从高到低展示，可查看全部月份或单个月份。</HelpTip></h2>
      <NativeMonthSelect label="重度用户月份" value={month} months={months} onChange={setMonth} />
    </div>
    {users.length ? <ol className="mt-3 grid list-none gap-1" aria-label="重度用户Token排名">
      {users.map((user) => <li key={user.id} className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2 text-xs">
        <span className="truncate text-right font-medium" title={user.name}>{user.name}</span>
        <div className="relative h-5 overflow-hidden rounded-md bg-[var(--surface-muted)]">
          <span className="absolute inset-y-0 left-0 rounded-md bg-[var(--primary)]" style={{ width: `${Math.max(2, (user.tokens / maximum) * 100)}%` }} aria-hidden="true" />
          <span className="absolute inset-y-0 right-2 flex items-center tabular-nums text-[var(--ink)]">{compactNumber(user.tokens)}</span>
        </div>
      </li>)}
    </ol> : <Empty title="暂无重度用户数据" detail="当前月份没有可展示的成员 Token 数据。" />}
  </section>;
}

export function useMonthlyActualCost(auto: boolean) {
  const boot = useBootstrap();
  const currentMonth = platformDateParts(boot.generatedAt, boot.timezone).date.slice(0, 7);
  const filters: CostFilters = { month: currentMonth, department: "", platform: "", q: "", contributor: "", paymentMethod: "", status: "", completeness: "", registration: "", page: 1, pageSize: 20 };
  const state = useRemote((signal) => insightsApi.costs(filters, signal, "summary"), [currentMonth], auto, `${boot.scope}:department-costs:${currentMonth}`);
  return { currentMonth, state };
}

export function MonthlyActualCostCard({ cost }: { cost: ReturnType<typeof useMonthlyActualCost> }) {
  const { currentMonth, state } = cost;
  const [currency, setCurrency] = useState<CostCurrency>("cny");
  const trend = pastCostTrend(state.data?.trend || [], currentMonth);
  const cumulativeCost = convertCost(cumulativeActualCost(trend), currency);
  const symbol = currency === "cny" ? "¥" : "$";
  const currencyCode = currency === "cny" ? "CNY" : "USD";
  const chartValues = trend.map((point) => convertCost(point.actualCost, currency));
  const theme = cartesianTheme(54);
  const colors = chartColors();
  return <section className="panel h-[340px]">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 className="m-0 text-base font-semibold">支出成本</h2>
        <p className="mb-0 mt-0.5 text-xs leading-5 muted">仅统计平台已纳管账号，成本按 AI 订阅价格估算。</p>
      </div>
      <div className="flex flex-wrap items-start justify-end gap-3">
        <div className="flex items-baseline gap-2 whitespace-nowrap"><span className="text-xs muted">累计成本</span><strong className="tabular-nums text-lg font-semibold text-[var(--ink)]">{cumulativeCost.toLocaleString("zh-CN", { style: "currency", currency: currencyCode, minimumFractionDigits: 2, maximumFractionDigits: 2 })}</strong></div>
        <div className="flex items-center gap-1"><HelpTip label="人民币汇率说明">人民币按固定汇率 1 USD = 6.7 CNY 换算。</HelpTip><Segmented value={currency} onChange={setCurrency} label="支出成本币种" options={[{ value: "cny", label: "人民币" }, { value: "usd", label: "美元" }]} /></div>
      </div>
    </div>
    {state.loading && !state.data ? <Loading label="正在加载支出成本" /> : trend.length ? <Chart label="过往月份支出成本柱状图" height={260} option={{
      color: [palette[2]], textStyle: theme.textStyle,
      tooltip: { ...tooltipTheme(), trigger: "axis", axisPointer: { type: "shadow" }, valueFormatter: (value: unknown) => `${symbol}${Number(value || 0).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` },
      grid: { ...(theme.grid as object), top: 34, bottom: 48 },
      xAxis: { ...theme.xAxis, type: "category", data: trend.map((point) => monthLabel(point.month)), axisLabel: { ...(theme.xAxis.axisLabel as object), hideOverlap: true } },
      yAxis: { ...theme.yAxis, type: "value", axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: (value: unknown) => `${symbol}${compactNumber(value)}` } },
      series: [{ name: "真实支出", type: "bar", barMaxWidth: 44, data: chartValues, itemStyle: { borderRadius: [5, 5, 1, 1] }, label: { show: true, position: "top", color: colors.ink, fontSize: 12, formatter: (params: { value?: unknown }) => `${symbol}${compactNumber(params.value)}` } }],
    }} /> : <Empty title="暂无已结束月份成本" detail={state.error || "当前没有可展示的过往月份支出数据。"} />}
  </section>;
}
