import { useState } from "react";
import { platformDateParts, subtractPlatformDays } from "../lib/format";
import type { FilterState } from "../lib/types";
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
  const state = useRemote(
      (s) => insightsApi.gateway(filters, s),
      [filters],
      auto,
    ),
    d = state.data;
  const preset = (n: number) =>
    setFilters((v) => ({
      ...v,
      from: subtractPlatformDays(platformToday.date, n - 1),
      to: platformToday.date,
    }));
  return (
    <div className="page-stack grid gap-6">
      <div>
        <h1 className="m-0 text-2xl font-semibold">网关数据</h1>
        <p className="mb-0 mt-1 text-sm muted">
          最终模型调用口径；内部重试归属同一调用，进行中不计入结果。
        </p>
      </div>
      <section className="panel p-4">
        <div className="mb-3 flex gap-2">
          <Button variant="secondary" onClick={() => preset(1)}>
            今天
          </Button>
          <Button variant="secondary" onClick={() => preset(7)}>
            近7天
          </Button>
          <Button variant="secondary" onClick={() => preset(30)}>
            近30天
          </Button>
        </div>
        <AnalyticsFilters
          value={filters}
          onChange={setFilters}
          models={boot.models}
          departments={boot.departments}
          showModels={false}
          showDepartments
          granularities={["hour", "day", "week", "month"]}
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
            <section>
              <h2 className="text-base">请求质量</h2>
              <div className="metric-strip grid sm:grid-cols-2 xl:grid-cols-5">
                <MetricCard
                  label="请求总次数"
                  value={d.totalCalls}
                  detail="成功 + 失败"
                />
                <MetricCard label="失败次数" value={d.failedCalls} />
                <MetricCard
                  label="成功率"
                  value={d.successRate === null ? null : d.successRate * 100}
                  unit="%"
                />
                <MetricCard
                  label="模型平均处理时长"
                  value={d.modelDuration.value}
                  unit="ms"
                  detail="最终成功上游尝试，网关观测值"
                />
                <MetricCard
                  label="网关平均处理时长"
                  value={d.gatewayDuration.value}
                  unit="ms"
                  detail="转发前，含排队"
                />
              </div>
              <div className="panel mt-4 p-4">
                <TimeSeriesChart
                  data={d.series}
                  metric="successRate"
                  mode="line"
                  percent
                  label="请求成功率趋势"
                />
              </div>
            </section>
            <section>
              <h2 className="text-base">用户分析</h2>
              <div className="metric-strip grid sm:grid-cols-2 xl:grid-cols-6">
                <MetricCard label="期末总用户数" value={d.totalUsers} />
                <MetricCard label="新增用户数" value={d.newUsers} />
                <MetricCard
                  label="高频用户"
                  value={d.frequency.high}
                  detail="> 200 次"
                />
                <MetricCard
                  label="中频用户"
                  value={d.frequency.medium}
                  detail="20–200 次"
                />
                <MetricCard
                  label="低频用户"
                  value={d.frequency.low}
                  detail="0–19 次"
                />                <MetricCard label="未分类用户" value={d.frequency.unclassified} detail="采集不完整，不并入低频" />

              </div>
              <div className="panel mt-4 p-4">
                <TimeSeriesChart
                  data={d.userSeries}
                  metric="users"
                  mode="line"
                  label="累计用户数趋势"
                />
              </div>
            </section>
            <div className="grid gap-6 xl:grid-cols-2">
              <section className="panel p-4">
                <h2 className="mt-0 text-base">用户转化漏斗</h2><CoverageBanner coverage={d.retentionCoverage} />
                <p className="text-xs muted">
                  按结束日期累计快照；第N日及以后回访。待观察和覆盖缺失不计作流失。
                </p>
                {d.funnel.length ? (
                  <Chart
                    label="真实递减用户留存漏斗"
                    height={390}
                    option={buildFunnelOption(d.funnel)}
                  />
                ) : (
                  <Empty />
                )}
                              {d.funnel.some(layer=>layer.count===null)&&<p className="mb-0 text-sm muted">{d.funnel.filter(layer=>layer.count===null).map(layer=>layer.label).join("、")}：历史数据不足，暂无法判断。</p>}
              </section>
              <section className="panel p-4">
                <h2 className="mt-0 text-base">部门模型偏好</h2><CoverageBanner coverage={d.preferenceCoverage} />
                <p className="text-xs muted">
                  每部门完整区间内调用构成，横向条合计100%。
                </p>
                {d.preferences.length ? (
                  <Chart
                    label="部门模型偏好100%堆叠图"
                    height={390}
                    option={buildPreferenceOption(d.preferences)}
                  />
                ) : (
                  <Empty />
                )}
              </section>
            </div>
          </>
        )
      )}
    </div>
  );
}
