import { useState } from "react";
import { Gauge } from "lucide-react";
import { platformDateParts } from "../lib/format";
import type { DepartmentOverview, FilterState } from "../lib/types";
import { useBootstrap } from "../features/BootstrapContext";
import { AnalyticsFilters } from "../features/Filters";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import { ErrorBanner, Loading } from "../components/ui/States";
import { MetricCard } from "../components/ui/MetricCard";
import { HelpTip } from "../components/ui/HelpTip";
import { Select } from "../components/ui/Controls";
import { CategoryTokenChart } from "../components/charts/CategoryTokenChart";
import { departmentMetricGroups } from "../features/departmentMetrics";
import { HeavyUsersCard, ModelFamilyDistributionCard, MonthlyActualCostCard, UserDepartmentDistributionCard, UserGrowthCard, useMonthlyActualCost } from "./DepartmentAnalyticsCharts";

const departmentHistoryStart = "2026-07-01";
type DepartmentMetricGroups = ReturnType<typeof departmentMetricGroups>;

function DepartmentOverviewPanel({ groups }: { groups: DepartmentMetricGroups }) {
  return <section className="panel department-overview grid overflow-hidden !p-0 md:grid-cols-3" aria-label="部门概览">
    {groups.map(group => <div className="department-metric-group" key={group.title}>
      <h2 className="m-0 px-4 pt-3 text-xs font-medium muted">{group.title}</h2>
      <div className="department-metric-lead"><MetricCard {...group.primary} /></div>
      <div className="grid grid-cols-2 border-t border-[var(--border)]">
        {group.secondary.map(item => <MetricCard key={item.label} {...item} />)}
      </div>
    </div>)}
  </section>;
}

export function DepartmentPage({ auto }: { auto: boolean }) {
  const boot = useBootstrap();
  const platformToday = platformDateParts(boot.generatedAt, boot.timezone);
  const [filters, setFilters] = useState<FilterState>(() => ({
      from: departmentHistoryStart,
      to: platformToday.date,
      granularity: "month",
      models: [],
      departments: [],
    }));
  const overviewKey = `${boot.scope}:department-overview:${JSON.stringify(filters)}`;
  const state = useRemote(
    (signal) => insightsApi.departmentsOverview(filters, signal),
    [filters],
    auto,
    overviewKey,
  );
  const growthState = useRemote(
    (signal) => insightsApi.platformUserGrowth(filters.from, filters.to, signal),
    [filters.from, filters.to],
    auto,
    `${boot.scope}:platform-user-growth:${filters.from}:${filters.to}`,
  );
  const monthlyCost = useMonthlyActualCost(auto);
  const data = state.data;
  const metricGroups = data ? departmentMetricGroups(data) : null;

  return (
    <div className="page-stack grid gap-6">
      <section className="filter-toolbar">
        <AnalyticsFilters value={filters} onChange={setFilters} models={boot.models} departments={boot.departments} showDepartments />
        {state.error && <div className="mt-3"><ErrorBanner message={state.error} retry={state.refresh} stale={!!data} /></div>}
      </section>
      {state.loading && !data ? <Loading /> : data && (
        <>
          <div className="grid items-stretch gap-6 xl:grid-cols-2">
            <UserGrowthCard series={growthState.data?.series} loading={growthState.loading} error={growthState.error || undefined} retry={growthState.refresh} />
            <UserDepartmentDistributionCard distribution={data.memberDistribution} />
          </div>
          {state.dataKey === overviewKey && <DepartmentDetails filters={filters} auto={auto} overview={data} metricGroups={metricGroups || []} monthlyCost={monthlyCost} />}
        </>
      )}
    </div>
  );
}

function DepartmentDetails({ filters, auto, overview, metricGroups, monthlyCost }: { filters: FilterState; auto: boolean; overview: DepartmentOverview; metricGroups: DepartmentMetricGroups; monthlyCost: ReturnType<typeof useMonthlyActualCost> }) {
  const boot = useBootstrap();
  const [performanceModel, setPerformanceModel] = useState("");
  const state = useRemote(
    (signal) => insightsApi.departments(filters, performanceModel, signal),
    [filters, performanceModel],
    auto,
    `${boot.scope}:department-details:${JSON.stringify(filters)}:${performanceModel}`,
  );
  const data = state.data;
  if (state.loading && !data) return <section className="panel"><Loading label="正在加载详细分析" /></section>;
  if (!data) return state.error ? <ErrorBanner message={state.error} retry={state.refresh} /> : null;
  return <>
          {state.error && <ErrorBanner message={state.error} retry={state.refresh} stale />}
          <div className="grid items-stretch gap-6 xl:grid-cols-2">
            <section className="panel h-full"><CategoryTokenChart data={overview.series} dimension="departments" granularity={filters.granularity} aggregate showMetricControl={false} title="研发部整体Token消耗" /></section>
            <ModelFamilyDistributionCard series={data.series} />
          </div>
          <div className="grid items-stretch gap-6 xl:grid-cols-2">
            <HeavyUsersCard data={data} />
            <MonthlyActualCostCard cost={monthlyCost} />
          </div>
          {metricGroups.length > 0 && <DepartmentOverviewPanel groups={metricGroups} />}
          <section className="panel">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div className="flex min-w-0 items-center gap-3">
                <span className="grid h-9 w-9 shrink-0 place-items-center rounded-[9px] bg-[var(--primary-soft)] text-[var(--primary)]"><Gauge aria-hidden="true" className="h-4 w-4" /></span>
                <h2 className="m-0 flex items-center gap-2 text-base font-semibold">部门模型性能<HelpTip label="部门模型性能说明">聚合所选日期与部门范围内的有效样本。</HelpTip></h2>
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
  </>;
}
