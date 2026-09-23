import { useState } from "react";
import { Activity, ChartNoAxesCombined, GitMerge, Users } from "lucide-react";
import { platformDateParts, subtractPlatformDays } from "../lib/format";
import type { Coverage, FilterState } from "../lib/types";
import { useBootstrap } from "../features/BootstrapContext";
import { AnalyticsFilters } from "../features/Filters";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import { CoverageBanner, Empty, ErrorBanner, Loading } from "../components/ui/States";
import { MetricCard } from "../components/ui/MetricCard";
import { TimeSeriesChart } from "../components/charts/TimeSeriesChart";
import { Chart } from "../components/charts/Chart";
import { buildFunnelOption, buildPreferenceOption } from "../features/chartOptions";
import { Button } from "../components/ui/Button";

export function GatewayPage({ auto }: { auto: boolean }) {
  const boot = useBootstrap();
  const platformToday = platformDateParts(boot.generatedAt, boot.timezone);
  const [filters, setFilters] = useState<FilterState>(() => ({
    from: subtractPlatformDays(platformToday.date, 6),
    to: platformToday.date,
    granularity: "day",
    models: [],
    departments: [],
  }));
  const state = useRemote((signal) => insightsApi.gateway(filters, signal), [filters], auto);
  const data = state.data;
  const applyPreset = (days: number) => setFilters((value) => ({
    ...value,
    from: subtractPlatformDays(platformToday.date, days - 1),
    to: platformToday.date,
  }));
  const isPreset = (days: number) => filters.from === subtractPlatformDays(platformToday.date, days - 1) && filters.to === platformToday.date;

  return (
    <div className="page-stack grid gap-6">
      <header className="page-heading">
        <div>
          <h1 className="m-0 text-2xl font-semibold">网关数据</h1>
          <p className="mb-0 mt-1 text-sm muted">最终模型调用口径；内部重试归属同一调用，进行中不计入结果。</p>
        </div>
      </header>

      <section className="panel">
        <div className="flex flex-wrap items-end gap-4">
          <div>
            <div className="mb-2 flex items-center gap-2 text-sm font-semibold"><ChartNoAxesCombined aria-hidden="true" className="h-4 w-4 text-[var(--primary)]" />分析范围</div>
            <div className="flex gap-2" aria-label="快捷日期范围">
              {[{ days: 1, label: "今天" }, { days: 7, label: "近7天" }, { days: 30, label: "近30天" }].map((preset) => (
                <Button key={preset.days} variant={isPreset(preset.days) ? "primary" : "secondary"} onClick={() => applyPreset(preset.days)}>{preset.label}</Button>
              ))}
            </div>
          </div>
          <div className="min-w-0 flex-1"><AnalyticsFilters value={filters} onChange={setFilters} models={boot.models} departments={boot.departments} showModels={false} showDepartments granularities={["hour", "day", "week", "month"]} /></div>
        </div>
        {state.error && <div className="mt-3"><ErrorBanner message={state.error} retry={state.refresh} stale={!!data} /></div>}
      </section>

      {state.loading && !data ? <Loading /> : data && (
        <>
          <CoverageBanner coverage={data.coverage} />
          <section className="panel">
            <SectionHeading icon={<Activity aria-hidden="true" className="h-4 w-4" />} title="请求质量" detail="成功率基于已有明确结果的模型调用。" />
            <div className="mt-4 grid overflow-hidden rounded-[10px] border border-[var(--border)] md:grid-cols-3 xl:grid-cols-5">
              <MetricCard label="请求总次数" value={data.totalCalls} detail="成功 + 失败" />
              <MetricCard label="失败次数" value={data.failedCalls} detail="拒绝、报错、超时、中断或取消" />
              <MetricCard label="成功率" value={data.successRate === null ? null : data.successRate * 100} unit="%" />
              <MetricCard label="模型平均处理时长" value={data.modelDuration.value} unit="ms" detail="最终成功上游尝试，网关观测值" />
              <MetricCard label="网关平均处理时长" value={data.gatewayDuration.value} unit="ms" detail="首次转发前，含排队" />
            </div>
            <div className="mt-4 border-t border-[var(--border)] pt-3">
              <div className="text-xs font-semibold muted">请求成功率趋势</div>
              <TimeSeriesChart data={data.series} metric="successRate" mode="line" percent label="请求成功率趋势" height={300} />
            </div>
            <p className="mb-0 mt-2 text-xs muted">全部部门口径保留身份未知及已删除用户历史，因此可能大于当前各部门之和。</p>
          </section>

          <section className="panel">
            <SectionHeading icon={<Users aria-hidden="true" className="h-4 w-4" />} title="用户分析" detail="频次按整个所选区间累计；切换粒度不会重新分类。" />
            <div className="mt-3"><CoverageBanner coverage={data.frequency.coverage} /></div>
            <div className="mt-4 grid overflow-hidden rounded-[10px] border border-[var(--border)] md:grid-cols-3 xl:grid-cols-6">
              <MetricCard label="期末总用户数" value={data.totalUsers} detail="截至所选期末" />
              <MetricCard label="新增用户数" value={data.newUsers} detail="区间内新建账号" />
              <MetricCard label="高频用户" value={data.frequency.high} detail="> 200 次" />
              <MetricCard label="中频用户" value={data.frequency.medium} detail="20–200 次" />
              <MetricCard label="低频用户" value={data.frequency.low} detail="0–19 次，含已知未使用者" />
              <MetricCard label="未分类用户" value={data.frequency.unclassified} detail="采集不完整，不并入低频" />
            </div>
            <div className="mt-4 border-t border-[var(--border)] pt-3">
              <div className="text-xs font-semibold muted">期末累计用户趋势</div>
              <TimeSeriesChart data={data.userSeries} metric="users" mode="line" label="累计用户数趋势" height={300} />
            </div>
          </section>

          <div className="grid items-stretch gap-6 xl:grid-cols-12">
            <section className="panel xl:col-span-6">
              <SectionHeading icon={<GitMerge aria-hidden="true" className="h-4 w-4" />} title="用户转化漏斗" detail="按结束日期累计快照；第N日及以后回访。" />
              <div className="mt-3"><CoverageBanner coverage={data.retentionCoverage} /></div>
              {data.funnel.length ? <Chart label="真实递减用户留存漏斗" height={390} option={buildFunnelOption(data.funnel)} /> : <Empty detail={emptyDetail(data.retentionCoverage, "当前筛选范围没有可展示的留存数据。") } />}
              {data.funnel.some((layer) => layer.count === null) && (
                <div className="rounded-[9px] border border-[var(--border)] bg-[var(--surface-subtle)] px-3 py-2 text-xs muted">
                  {data.funnel.filter((layer) => layer.count === null).map((layer) => layer.label).join("、")}：待观察或历史覆盖不足，暂无法判断；未作为流失或 0 人处理。
                </div>
              )}
            </section>
            <section className="panel xl:col-span-6">
              <SectionHeading icon={<ChartNoAxesCombined aria-hidden="true" className="h-4 w-4" />} title="部门模型偏好" detail="每个部门按完整区间的调用构成展示，条形合计100%。" />
              <div className="mt-3"><CoverageBanner coverage={data.preferenceCoverage} /></div>
              {data.preferences.length ? <Chart label="部门模型偏好100%堆叠图" height={390} option={buildPreferenceOption(data.preferences)} /> : <Empty detail={emptyDetail(data.preferenceCoverage, "当前筛选范围没有部门模型调用。") } />}
            </section>
          </div>
        </>
      )}
    </div>
  );
}

function SectionHeading({ icon, title, detail }: { icon: React.ReactNode; title: string; detail: string }) {
  return (
    <div className="flex items-start gap-3">
      <span className="grid h-9 w-9 shrink-0 place-items-center rounded-[9px] bg-[var(--primary-soft)] text-[var(--primary)]">{icon}</span>
      <div><h2 className="m-0 text-base font-semibold">{title}</h2><p className="mb-0 mt-1 text-xs muted">{detail}</p></div>
    </div>
  );
}

function emptyDetail(coverage: Coverage, empty: string) {
  if (coverage.state === "uncollected") return "该时段尚未采集。";
  if (coverage.state === "partial") return "尚无完整观测数据。";
  if (coverage.state === "unavailable") return "相关数据暂不可用。";
  return empty;
}
