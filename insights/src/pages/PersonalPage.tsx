import { useState } from "react";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import { useBootstrap } from "../features/BootstrapContext";
import { AnalyticsFilters } from "../features/Filters";
import {
  dateTime,
  money,
  platformDateParts,
  subtractPlatformDays,
} from "../lib/format";
import type { ChartMode, ErrorLog, FilterState, UsageLog } from "../lib/types";
import { MetricCard } from "../components/ui/MetricCard";
import {
  CoverageBanner,
  Empty,
  ErrorBanner,
  Loading,
} from "../components/ui/States";
import { Segmented, Select } from "../components/ui/Controls";
import { TimeSeriesChart } from "../components/charts/TimeSeriesChart";
import { ModelDonut } from "../components/charts/ModelDonut";
import { Chart } from "../components/charts/Chart";
import { Button } from "../components/ui/Button";
import { buildHeatmapOption, heatmapSelectionDate } from "../features/chartOptions";
export function PersonalPage({ auto }: { auto: boolean }) {
  const { models, timezone, generatedAt } = useBootstrap();
  const platformToday = platformDateParts(generatedAt, timezone);
  const [filters, setFilters] = useState<FilterState>(() => ({
      from: subtractPlatformDays(platformToday.date, 6),
      to: platformToday.date,
      granularity: "day",
      models: [],
      departments: [],
    })),
    [year, setYear] = useState(platformToday.year),
    [mode, setMode] = useState<ChartMode>("line"),
    [metric, setMetric] = useState<
      "totalTokens" | "outputTokens" | "cacheHitRate"
    >("totalTokens"),
    [tab, setTab] = useState<"usage" | "errors">("usage"),
    [pageSize, setPageSize] = useState(20),
    [cursors, setCursors] = useState<Array<string | undefined>>([undefined]);
  const overview = useRemote((s) => insightsApi.personalOverview(s), [], auto),
    heatmap = useRemote(
      (s) => insightsApi.personalHeatmap(year, s),
      [year],
      auto,
    ),
    analytics = useRemote(
      (s) => insightsApi.personalAnalytics(filters, s),
      [filters],
      auto,
    ),
    logs = useRemote(
      (s) => insightsApi.logs(tab, filters, cursors.at(-1), pageSize, s),
      [tab, filters, cursors, pageSize],
      false,
    );
  const selectDay = (p: unknown) => {
    const date = heatmapSelectionDate(p, heatmap.data?.days || []);
    if (date) setFilters((v) => ({ ...v, from: date, to: date }));
  };
  return (
    <div className="page-stack grid min-w-0 max-w-full gap-6">
      <PageTitle
        title="个人数据"
        detail={`统计记录时间 · ${timezone} · 历史默认近7个自然日`}
      />
      {overview.error && (
        <ErrorBanner
          message={overview.error}
          retry={overview.refresh}
          stale={!!overview.data}
        />
      )}{" "}
      {overview.loading && !overview.data ? (
        <Loading />
      ) : (
        overview.data && (
          <>
            <CoverageBanner coverage={overview.data.coverage} />
            <section>
              <h2 className="text-base font-semibold">当天概览</h2>
              <div className="metric-strip today-overview-grid mt-3">
                <MetricCard
                  label="今天总Token"
                  value={overview.data.todayTokens.total}
                  detail={`输入 ${overview.data.todayTokens.input?.toLocaleString() ?? "—"} · 输出 ${overview.data.todayTokens.output?.toLocaleString() ?? "—"} · 缓存写 ${overview.data.todayTokens.cacheWrite?.toLocaleString() ?? "—"} · 缓存读 ${overview.data.todayTokens.cacheRead?.toLocaleString() ?? "—"}`}
                />
                {overview.data.subscriptions.length ? (
                  overview.data.subscriptions.map((s) => (
                    <div className="overview-summary-cell" key={s.id}>
                      <div className="text-sm font-medium">{s.name}</div>
                      <div className="mt-2 text-xl font-semibold">
                        {money(s.used)} {s.currency}
                      </div>
                      <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--surface-subtle)]">
                        <div
                          className="h-full bg-[var(--primary)]"
                          style={{
                            width: s.limit
                              ? `${Math.min(100, (s.used / s.limit) * 100)}%`
                              : "100%",
                          }}
                        />
                      </div>
                      <div className="mt-2 text-xs muted">
                        {s.limit === null
                          ? "额度不限"
                          : s.status === "exceeded"
                            ? `已超限，额度 ${money(s.limit)}`
                            : `剩余 ${s.remaining === null ? "—" : money(s.remaining)}`}{" "}
                        {s.resetsAt && `· ${dateTime(s.resetsAt, timezone)} 重置`}
                      </div>
                    </div>
                  ))
                ) : (
                  <div className="overview-summary-cell">
                    <strong>无有效订阅</strong>
                    <p className="mb-0 text-sm muted">
                      当天Token仍按网关记录展示。
                    </p>
                  </div>
                )}
              </div>
            </section>
          </>
        )
      )}
      <section className="panel min-w-0 max-w-full p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="m-0 text-base font-semibold">年度Token热力图</h2>
            <p className="mb-0 mt-1 text-xs muted">
              全部模型；色阶跨保留年份一致。点击日期定位下方历史数据。
            </p>
          </div>
          <Select
            value={year}
            onChange={(e) => setYear(Number(e.target.value))}
          >
            {[0, 1, 2].map((n) => (
              <option key={n} value={platformToday.year - n}>
                {platformToday.year - n}
              </option>
            ))}
          </Select>
        </div>
        {heatmap.error && (
          <ErrorBanner message={heatmap.error} retry={heatmap.refresh} stale={!!heatmap.data} />
        )}
        {heatmap.loading && !heatmap.data ? (
          <Loading />
        ) : heatmap.data ? (
          <>
            <CoverageBanner coverage={heatmap.data.coverage} />
            <Chart
              label="年度Token热力图"
              height={210}
              onEvents={{ click: selectDay }}
              option={buildHeatmapOption(heatmap.data.days, heatmap.data.thresholds, year)}
            />            <div className="mt-2 flex flex-wrap justify-end gap-x-4 gap-y-2 text-sm muted" aria-label="热力图数据状态">
              <span className="inline-flex items-center gap-1.5"><span className="h-3 w-3 rounded-sm border border-[var(--border)] bg-[var(--surface-subtle)]" aria-hidden />零用量</span>
              <span className="inline-flex items-center gap-1.5"><span className="h-3 w-3 rounded-sm bg-[var(--border)]" aria-hidden />未采集</span>
              <span className="inline-flex items-center gap-1.5"><span className="h-3 w-3 rounded-sm border border-dashed border-[var(--border-strong)]" aria-hidden />未来</span>
            </div>
          </>
        ) : heatmap.error ? null : (
          <Empty />
        )}
      </section>
      <section className="panel min-w-0 max-w-full p-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="m-0 text-base font-semibold">历史分析</h2>
            <p className="mb-0 mt-1 text-xs muted">
              日均包含真实零消耗日并排除未来日期。
            </p>
          </div>
          <AnalyticsFilters
            value={filters}
            onChange={setFilters}
            models={models}
          />
        </div>
        {analytics.error && (
          <div className="mt-4">
            <ErrorBanner
              message={analytics.error}
              retry={analytics.refresh}
              stale={!!analytics.data}
            />
          </div>
        )}
        {analytics.loading && !analytics.data ? (
          <Loading />
        ) : (
          analytics.data && (
            <>
              <CoverageBanner coverage={analytics.data.coverage} />
              <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
                <MetricCard
                  label="活跃天数"
                  value={analytics.data.activeDays}
                />
                <MetricCard
                  label="总Token"
                  value={analytics.data.totalTokens}
                />
                <MetricCard
                  label="日均Token"
                  value={analytics.data.averageDailyTokens}
                  formula="总Token ÷ 所选自然日数"
                />
                <MetricCard
                  label="请求次数"
                  value={analytics.data.requests}
                  detail="用量记录条数"
                />
                <MetricCard
                  label="输出Token"
                  value={analytics.data.outputTokens}
                />
                <MetricCard
                  label="缓存命中率"
                  value={
                    analytics.data.cacheHitRate === null
                      ? null
                      : analytics.data.cacheHitRate * 100
                  }
                  unit="%"
                  formula="Σ缓存读 ÷ (Σ输入 + Σ缓存写 + Σ缓存读)"
                />
              </div>
              <div className="mt-5 flex flex-wrap justify-between gap-3">
                <Segmented
                  value={metric}
                  onChange={setMetric}
                  label="时间指标"
                  options={[
                    { value: "totalTokens", label: "总Token" },
                    { value: "outputTokens", label: "输出Token" },
                    { value: "cacheHitRate", label: "缓存命中率" },
                  ]}
                />
                <Segmented
                  value={mode}
                  onChange={setMode}
                  label="图表模式"
                  options={[
                    { value: "line", label: "折线趋势" },
                    { value: "bar", label: "柱状分布" },
                  ]}
                />
              </div>
              <TimeSeriesChart
                data={analytics.data.series}
                metric={metric}
                mode={mode}
                percent={metric === "cacheHitRate"}
                label="个人历史时间趋势"
              />
              <div className="mt-5 border-t border-[var(--border)] pt-4">
                <h3 className="text-sm font-semibold">模型调用分布</h3>
                {analytics.data.models.length ? (
                  <ModelDonut models={analytics.data.models} />
                ) : (
                  <Empty />
                )}
              </div>
            </>
          )
        )}
      </section>
      <section className="panel min-w-0 max-w-full p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Segmented
            value={tab}
            onChange={(v) => {
              setTab(v);
              setCursors([undefined]);
            }}
            label="日志类型"
            options={[
              { value: "usage", label: "用量日志" },
              { value: "errors", label: "错误日志" },
            ]}
          />
          <label className="text-xs muted">
            每页{" "}
            <Select
              value={pageSize}
              onChange={(e) => {
                setPageSize(Number(e.target.value));
                setCursors([undefined]);
              }}
            >
              {[20, 50, 100].map((n) => (
                <option key={n}>{n}</option>
              ))}
            </Select>
          </label>
        </div>
        {logs.error && (
          <ErrorBanner
            message={logs.error}
            retry={logs.refresh}
            stale={!!logs.data}
          />
        )}{" "}
        {logs.loading && !logs.data ? (
          <Loading />
        ) : logs.data?.items.length ? (
          <div className="mt-3 min-w-0 max-w-full overflow-x-auto">
            <table className="w-full min-w-[900px] text-sm">
              <thead className="text-left muted">
                <tr>
                  {tab === "usage" ? (
                    <>
                      <th>统计时间</th>
                      <th>部门</th>
                      <th>模型</th>
                      <th>API Key</th>
                      <th className="text-right">Token I/O/W/R</th>
                      <th className="text-right">耗时 / TTFT</th>
                    </>
                  ) : (
                    <>
                      <th>时间</th>
                      <th>模型</th>
                      <th>错误类型</th>
                      <th>简要原因</th>
                    </>
                  )}
                  <th>元数据</th>
                </tr>
              </thead>
              <tbody>
                {logs.data.items.map((row: UsageLog | ErrorLog) => (
                  <LogRow key={row.id} row={row} kind={tab} timezone={timezone} />
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty title="没有日志" />
        )}
        <div className="mt-3 flex justify-end gap-2">
          <Button
            variant="secondary"
            disabled={cursors.length <= 1}
            onClick={() => setCursors((items) => items.slice(0, -1))}
          >
            上一页
          </Button>
          <span className="self-center text-sm muted">
            第 {cursors.length} 页
          </span>
          <Button
            variant="secondary"
            disabled={!logs.data?.hasMore || !logs.data.nextCursor}
            onClick={() =>
              logs.data?.nextCursor &&
              setCursors((items) => [...items, logs.data!.nextCursor])
            }
          >
            下一页
          </Button>
          <Button variant="secondary" onClick={logs.refresh}>
            刷新日志
          </Button>
        </div>
      </section>
    </div>
  );
}
function LogRow({
  row,
  kind,
  timezone,
}: {
  row: UsageLog | ErrorLog;
  kind: "usage" | "errors";
  timezone: string;
}) {
  const [open, setOpen] = useState(false);
  if (kind === "usage") {
    const u = row as UsageLog;
    return (
      <>
        <tr className="border-t border-[var(--border)]">
          <td className="py-3">{dateTime(u.recordedAt, timezone)}</td>
          <td>{u.department}</td>
          <td>{u.model}</td>
          <td>{u.apiKeyName}</td>
          <td className="text-right tabular-nums">
            {u.inputTokens}/{u.outputTokens}/{u.cacheWriteTokens}/
            {u.cacheReadTokens}
          </td>
          <td className="text-right">
            {u.durationMs ?? "—"} / {u.ttftMs ?? "—"} ms
          </td>
          <td>
            <button
              className="log-detail-button text-[var(--primary)]"
              aria-expanded={open}
              onClick={() => setOpen((v) => !v)}
            >
              {open ? "收起" : "展开"}
            </button>
          </td>
        </tr>
        {open && (
          <tr>
            <td colSpan={7}>
              <pre className="max-h-48 overflow-auto rounded bg-[var(--surface-subtle)] p-3 text-xs">
                {JSON.stringify(u.metadata, null, 2)}
              </pre>
            </td>
          </tr>
        )}
      </>
    );
  }
  const e = row as ErrorLog;
  return (
    <>
      <tr className="border-t border-[var(--border)]">
        <td className="py-3">{dateTime(e.recordedAt, timezone)}</td>
        <td>{e.model}</td>
        <td>{e.type}</td>
        <td>{e.reason}</td>
        <td><button className="log-detail-button text-[var(--primary)]" aria-expanded={open} onClick={()=>setOpen(v=>!v)}>{open?"收起":"展开"}</button></td>
      </tr>
      {open && <tr><td colSpan={5}><pre className="max-h-48 overflow-auto rounded bg-[var(--surface-subtle)] p-3 text-xs">{JSON.stringify(e.metadata,null,2)}</pre></td></tr>}
    </>
  );
}
function PageTitle({ title, detail }: { title: string; detail: string }) {
  return (
    <div>
      <h1 className="m-0 text-2xl font-semibold">{title}</h1>
      <p className="mb-0 mt-1 text-sm muted">{detail}</p>
    </div>
  );
}
