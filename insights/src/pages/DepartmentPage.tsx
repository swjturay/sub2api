import { useState } from "react";
import { platformDateParts, subtractPlatformDays } from "../lib/format";
import type { ChartMode, FilterState } from "../lib/types";
import { useBootstrap } from "../features/BootstrapContext";
import { AnalyticsFilters } from "../features/Filters";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import {
  CoverageBanner,
  Empty,
  ErrorBanner,
  Loading,
} from "../components/ui/States";
import { MetricCard } from "../components/ui/MetricCard";
import { ModelDonut } from "../components/charts/ModelDonut";
import { Segmented, Select } from "../components/ui/Controls";
import { TimeSeriesChart } from "../components/charts/TimeSeriesChart";
import { Chart, palette } from "../components/charts/Chart";
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
    (s) => insightsApi.departments(filters, performanceModel, s),
    [filters, performanceModel],
    auto,
  );
  const d = state.data;
  const metricGroups = d ? departmentMetricGroups(d) : null;
  return (
    <div className="page-stack grid gap-6">
      <div>
        <h1 className="m-0 text-2xl font-semibold">部门数据</h1>
        <p className="mb-0 mt-1 text-sm muted">
          按用户当前单一部门归集历史；停用成员保留，已删除成员排除。
        </p>
      </div>
      <section className="panel p-4">
        <AnalyticsFilters
          value={filters}
          onChange={setFilters}
          models={boot.models}
          departments={boot.departments}
          showDepartments
        />
        {state.error && (
          <div className="mt-3">
            <ErrorBanner
              message={state.error}
              retry={state.refresh}
              stale={!!d}
            />
          </div>
        )}
      </section>
      {state.loading && !d ? (
        <Loading />
      ) : (
        d && (
          <>
            <CoverageBanner coverage={d.coverage} />
            {metricGroups && (
              <div className="department-metric-bands grid gap-3">
                <div className="metric-strip department-metrics-primary grid xl:grid-cols-3">
                  {metricGroups.primary.map((item) => (
                    <MetricCard key={item.label} {...item} />
                  ))}
                </div>
                <div className="metric-strip department-metrics-secondary grid sm:grid-cols-2 xl:grid-cols-6">
                  {metricGroups.secondary.map((item) => (
                    <MetricCard key={item.label} {...item} />
                  ))}
                </div>
              </div>
            )}
            <section className="panel p-4">
              <div className="flex justify-between">
                <div><h2 className="m-0 text-base">指标趋势</h2><div className="mt-2"><Segmented value={metric} onChange={setMetric} label="趋势指标" options={[{ value: "totalTokens", label: "总Token" }, { value: "outputTokens", label: "输出Token" }, { value: "cacheHitRate", label: "缓存命中率" }]} /></div></div>
                <Segmented
                  value={mode}
                  onChange={setMode}
                  label="图表模式"
                  options={[
                    { value: "line", label: "折线" },
                    { value: "bar", label: "柱状" },
                  ]}
                />
              </div>
              <TimeSeriesChart
                data={d.series}
                metric={metric}
                mode={mode}
                label="部门指标趋势" percent={metric === "cacheHitRate"}
              />
            </section>
            <section className="panel p-4">
              <h2 className="mt-0 text-base">模型调用分布</h2>
              {d.models.length ? <ModelDonut models={d.models} /> : <Empty />}
            </section>
            <div className="grid gap-6 xl:grid-cols-2">
              <section className="panel p-4">
                <div className="flex justify-between">
                  <h2 className="m-0 text-base">消耗帕累托</h2>
                  <Segmented
                    value={pareto}
                    onChange={setPareto}
                    label="帕累托维度"
                    options={[
                      { value: "department", label: "部门" },
                      { value: "member", label: "成员" },
                    ]}
                  />
                </div>
                <Chart
                  label="Token消耗帕累托图"
                  height={360}
                  option={{
                    color: [palette[0], palette[2]],
                    tooltip: { trigger: "axis" },
                    legend: { data: ["总Token", "累计占比"] },
                    grid: { left: 70, right: 62, bottom: 90 },
                    xAxis: {
                      type: "category",
                      data: (pareto === "member" ? d.memberPareto : d.departmentPareto).map((x) => x.name),
                      axisLabel: { rotate: 35 },
                    },
                    yAxis: [
                      { type: "value" },
                      {
                        type: "value",
                        max: 100,
                        axisLabel: { formatter: "{value}%" },
                      },
                    ],
                    series: [
                      {
                        name: "总Token",
                        type: "bar",
                        data: (pareto === "member" ? d.memberPareto : d.departmentPareto).map((x) => x.tokens),
                      },
                      {
                        name: "累计占比",
                        type: "line",
                        yAxisIndex: 1,
                        data: (pareto === "member" ? d.memberPareto : d.departmentPareto).map((x) => x.cumulativeShare * 100),
                        markLine: { data: [{ yAxis: 80 }] },
                      },
                    ],
                    dataZoom: [
                      { type: "inside" },
                      { type: "slider", bottom: 15 },
                    ],
                  }}
                />
              </section>
              <section className="panel p-4">
                <h2 className="mt-0 text-base">成员 TOP10</h2>
                <Chart
                  label="成员Token TOP10"
                  height={360}
                  option={{
                    color: [palette[0]],
                    tooltip: { trigger: "axis" },
                    grid: { left: 120, right: 25 },
                    xAxis: { type: "value" },
                    yAxis: {
                      type: "category",
                      inverse: true,
                      data: d.topUsers.map((x) => x.name),
                    },
                    series: [
                      {
                        type: "bar",
                        data: d.topUsers.map((x) => x.tokens),
                        barMaxWidth: 22,
                      },
                    ],
                  }}
                />
              </section>
            </div>
            <section className="panel p-4">
              <div className="flex flex-wrap items-end justify-between gap-3">
                <div>
                  <h2 className="m-0 text-base">部门模型性能</h2>
                  <p className="mb-0 mt-1 text-xs muted">
                    独立模型选择，不受上方模型多选影响；TTFT/TPOT按有效原始样本计算。
                  </p>
                </div>
                <label className="grid gap-1 text-xs muted">
                  性能模型
                  <Select
                    value={performanceModel}
                    onChange={(e) => setPerformanceModel(e.target.value)}
                  >
                    <option value="">全部模型</option>
                    {boot.models.map((m) => (
                      <option value={m.id} key={m.id}>
                        {m.name} · {m.platform}
                      </option>
                    ))}
                  </Select>
                </label>
              </div>
              <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <MetricCard
                  label="平均TPM"
                  value={d.performance.tpm.value}
                  formula="窗口总Token ÷ 窗口分钟"
                />
                <MetricCard label="平均RPM" value={d.performance.rpm.value} />
                <MetricCard
                  label="平均TTFT"
                  value={d.performance.ttft.value}
                  unit="ms"
                  detail={`${d.performance.ttft.sampleCount ?? 0} 个有效样本`}
                />
                <MetricCard
                  label="估算TPOT"
                  value={d.performance.tpot.value}
                  unit="ms"
                  detail="逐请求估算后等权平均"
                />
              </div>
            </section>
          </>
        )
      )}
    </div>
  );
}
