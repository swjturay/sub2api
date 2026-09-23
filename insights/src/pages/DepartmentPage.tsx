import { useState } from "react";
import { Gauge } from "lucide-react";
import { platformDateParts, subtractPlatformDays } from "../lib/format";
import type { ChartMode, FilterState } from "../lib/types";
import { useBootstrap } from "../features/BootstrapContext";
import { AnalyticsFilters } from "../features/Filters";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import { CoverageBanner, Empty, ErrorBanner, Loading } from "../components/ui/States";
import { MetricCard } from "../components/ui/MetricCard";
import { ModelDonut } from "../components/charts/ModelDonut";
import { Segmented, Select } from "../components/ui/Controls";
import { TimeSeriesChart } from "../components/charts/TimeSeriesChart";
import { Chart, palette } from "../components/charts/Chart";
import { cartesianTheme, chartColors, compactNumber, legendTheme, tooltipTheme } from "../components/charts/chartTheme";
import { departmentMetricGroups } from "../features/departmentMetrics";

export function DepartmentPage({ auto }: { auto: boolean }) {
  const boot = useBootstrap();
  const platformToday = platformDateParts(boot.generatedAt, boot.timezone);
  const [filters, setFilters] = useState<FilterState>(() => ({
      from: subtractPlatformDays(platformToday.date, 6),
      to: platformToday.date,
      granularity: "day",
      models: [],
      departments: [],
    })),
    [performanceModel, setPerformanceModel] = useState(""),
    [pareto, setPareto] = useState<"department" | "member">("department"),
    [mode, setMode] = useState<ChartMode>("line"),
    [metric, setMetric] = useState<"totalTokens" | "outputTokens" | "cacheHitRate">("totalTokens");
  const state = useRemote(
    (signal) => insightsApi.departments(filters, performanceModel, signal),
    [filters, performanceModel],
    auto,
  );
  const data = state.data;
  const metricGroups = data ? departmentMetricGroups(data) : null;
  const paretoItems = data
    ? pareto === "member" ? data.memberPareto : data.departmentPareto
    : [];
  const paretoTheme = cartesianTheme(82);
  const rankingTheme = cartesianTheme(24);
  const colors = chartColors();

  return (
    <div className="page-stack grid gap-6">
      <header className="page-heading">
        <div>
          <h1 className="m-0 text-2xl font-semibold">部门数据</h1>
          <p className="mb-0 mt-1 text-sm muted">按用户当前单一部门归集历史；停用成员保留，已删除成员排除。</p>
        </div>
      </header>
      <section className="filter-toolbar">
        <AnalyticsFilters value={filters} onChange={setFilters} models={boot.models} departments={boot.departments} showDepartments />
        {state.error && <div className="mt-3"><ErrorBanner message={state.error} retry={state.refresh} stale={!!data} /></div>}
      </section>
      {state.loading && !data ? <Loading /> : data && (
        <>
          <CoverageBanner coverage={data.coverage} />
          {metricGroups && (
            <section className="panel overflow-hidden !p-0" aria-label="部门概览">
              <div className="department-metrics-primary grid md:grid-cols-3 [&_.metric-card]:!min-h-18 [&_.metric-card]:!py-2.5">
                  {metricGroups.primary.map((item) => <MetricCard key={item.label} {...item} detail={undefined} formula={item.formula || item.detail} />)}
              </div>
              <div className="grid border-t border-[var(--border)] md:grid-cols-3 xl:grid-cols-6 [&_.metric-card]:!min-h-16 [&_.metric-card]:!px-3 [&_.metric-card]:!py-2.5 [&_.metric-card>div:nth-child(2)]:!text-xl">
                {metricGroups.secondary.map((item) => <MetricCard key={item.label} {...item} />)}
              </div>
            </section>
          )}
          <div className="grid items-stretch gap-6 xl:grid-cols-12">
            <section className="panel xl:col-span-8">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-3">
                  <h2 className="m-0 text-base font-semibold">指标趋势</h2>
                  <Segmented value={metric} onChange={setMetric} label="趋势指标" options={[
                    { value: "totalTokens", label: "总Token" },
                    { value: "outputTokens", label: "输出Token" },
                    { value: "cacheHitRate", label: "缓存命中率" },
                  ]} />
                </div>
                <Segmented value={mode} onChange={setMode} label="图表模式" options={[
                  { value: "line", label: "折线" },
                  { value: "bar", label: "柱状" },
                ]} />
              </div>
              <TimeSeriesChart data={data.series} metric={metric} mode={mode} label="部门指标趋势" percent={metric === "cacheHitRate"} height={280} />
            </section>
            <section className="panel xl:col-span-4">
              <h2 className="m-0 text-base font-semibold">模型调用分布</h2>
              <p className="mb-0 mt-1 text-xs muted">按请求次数排序。</p>
              <div className="mt-2">{data.models.length ? <ModelDonut models={data.models} compact /> : <Empty />}</div>
            </section>
          </div>
          <div className="grid items-stretch gap-6 xl:grid-cols-12">
            <section className="panel xl:col-span-6">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h2 className="m-0 text-base font-semibold">消耗帕累托</h2>
                  <p className="mb-0 mt-1 text-xs muted">查看部门或成员的Token集中度。</p>
                </div>
                <Segmented value={pareto} onChange={setPareto} label="帕累托维度" options={[
                  { value: "department", label: "部门" },
                  { value: "member", label: "成员" },
                ]} />
              </div>
              {paretoItems.length ? <Chart label="Token消耗帕累托图" height={360} option={{
                color: [palette[0], palette[2]],
                textStyle: paretoTheme.textStyle,
                tooltip: { ...tooltipTheme(), trigger: "axis" },
                legend: { ...legendTheme(), data: ["总Token", "累计占比"] },
                grid: { ...(paretoTheme.grid as object), right: 54, bottom: 82 },
                xAxis: { ...paretoTheme.xAxis, type: "category", data: paretoItems.map((item) => item.name), axisLabel: { ...(paretoTheme.xAxis.axisLabel as object), rotate: 35 } },
                yAxis: [
                  { ...paretoTheme.yAxis, type: "value", axisLabel: { ...(paretoTheme.yAxis.axisLabel as object), formatter: compactNumber } },
                  { ...paretoTheme.yAxis, type: "value", max: 100, axisLabel: { ...(paretoTheme.yAxis.axisLabel as object), formatter: "{value}%" } },
                ],
                series: [
                  { name: "总Token", type: "bar", data: paretoItems.map((item) => item.tokens), barMaxWidth: 24, itemStyle: { borderRadius: [4, 4, 1, 1] } },
                  { name: "累计占比", type: "line", yAxisIndex: 1, smooth: 0.18, data: paretoItems.map((item) => item.cumulativeShare * 100), markLine: { symbol: "none", label: { formatter: "80%" }, data: [{ yAxis: 80 }] } },
                ],
                dataZoom: paretoItems.length > 10 ? [{ type: "inside" }, { type: "slider", bottom: 15 }] : [],
              }} /> : <Empty />}
            </section>
            <section className="panel xl:col-span-6">
              <h2 className="m-0 text-base font-semibold">成员 TOP10</h2>
              <p className="mb-0 mt-1 text-xs muted">按总Token降序。</p>
              {data.topUsers.length ? <Chart label="成员Token TOP10" height={360} option={{
                color: [palette[0]],
                textStyle: rankingTheme.textStyle,
                tooltip: { ...tooltipTheme(), trigger: "axis", axisPointer: { type: "shadow" } },
                grid: { ...(rankingTheme.grid as object), left: 10, right: 64, top: 24, bottom: 24 },
                xAxis: { ...rankingTheme.xAxis, type: "value", show: false },
                yAxis: { ...rankingTheme.yAxis, type: "category", inverse: true, data: data.topUsers.map((item) => item.name), axisLabel: { ...(rankingTheme.yAxis.axisLabel as object), color: colors.ink, width: 112, overflow: "truncate" }, axisTick: { show: false }, axisLine: { show: false } },
                series: [{ name: "总Token", type: "bar", data: data.topUsers.map((item) => item.tokens), barMaxWidth: 22, showBackground: true, backgroundStyle: { color: colors.surfaceSubtle, borderRadius: 4 }, itemStyle: { borderRadius: 4 }, label: { show: true, position: "right", color: colors.muted, fontSize: 12, formatter: (params: unknown) => compactNumber((params as { value?: unknown }).value) } }],
              }} /> : <Empty />}
            </section>
          </div>
          <section className="panel">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div className="flex min-w-0 items-start gap-3">
                <span className="grid h-9 w-9 shrink-0 place-items-center rounded-[9px] bg-[var(--primary-soft)] text-[var(--primary)]"><Gauge aria-hidden="true" className="h-4 w-4" /></span>
                <div>
                  <h2 className="m-0 text-base font-semibold">部门模型性能</h2>
                  <p className="mb-0 mt-1 text-xs muted">聚合所选日期与部门范围内的有效样本。</p>
                </div>
              </div>
              <label className="grid min-w-56 gap-1 text-xs muted">
                性能模型
                <Select value={performanceModel} onChange={(event) => setPerformanceModel(event.target.value)}>
                  <option value="">全部模型</option>
                  {boot.models.map((model) => <option value={model.id} key={model.id}>{model.name} · {model.platform}</option>)}
                </Select>
              </label>
            </div>
            <div className="mt-4 grid overflow-hidden rounded-[10px] border border-[var(--border)] md:grid-cols-2 xl:grid-cols-4">
              <MetricCard label="平均TPM" value={data.performance.tpm.value} formula="窗口总Token ÷ 窗口分钟" />
              <MetricCard label="平均RPM" value={data.performance.rpm.value} formula="用量记录数 ÷ 窗口分钟" />
              <MetricCard label="平均TTFT" value={data.performance.ttft.value} unit="ms" detail={`${data.performance.ttft.sampleCount ?? 0} 个有效样本`} />
              <MetricCard label="估算TPOT" value={data.performance.tpot.value} unit="ms" detail="逐请求估算后等权平均" />
            </div>
          </section>
        </>
      )}
    </div>
  );
}
