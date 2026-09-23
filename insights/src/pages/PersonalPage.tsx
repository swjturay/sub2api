import { useState } from "react";
import { CalendarDays, ChevronDown, ChevronUp, Clock3, WalletCards, Zap } from "lucide-react";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import { useBootstrap } from "../features/BootstrapContext";
import { AnalyticsFilters } from "../features/Filters";
import { dateTime, metric as formatMetric, money, platformDateParts, subtractPlatformDays } from "../lib/format";
import type { ChartMode, ErrorLog, FilterState, SubscriptionUsage, TokenBreakdown, UsageLog } from "../lib/types";
import { MetricCard } from "../components/ui/MetricCard";
import { CoverageBanner, Empty, ErrorBanner, Loading } from "../components/ui/States";
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
    [metric, setMetric] = useState<"totalTokens" | "outputTokens" | "cacheHitRate">("totalTokens"),
    [tab, setTab] = useState<"usage" | "errors">("usage"),
    [pageSize, setPageSize] = useState(20),
    [cursors, setCursors] = useState<Array<string | undefined>>([undefined]);
  const overview = useRemote((signal) => insightsApi.personalOverview(signal), [], auto);
  const heatmap = useRemote((signal) => insightsApi.personalHeatmap(year, signal), [year], auto);
  const analytics = useRemote((signal) => insightsApi.personalAnalytics(filters, signal), [filters], auto);
  const logs = useRemote(
    (signal) => insightsApi.logs(tab, filters, cursors.at(-1), pageSize, signal),
    [tab, filters, cursors, pageSize],
    false,
  );
  const updateFilters = (next: FilterState) => {
    setFilters(next);
    setCursors([undefined]);
  };
  const selectDay = (params: unknown) => {
    const date = heatmapSelectionDate(params, heatmap.data?.days || []);
    if (date) updateFilters({ ...filters, from: date, to: date });
  };

  return (
    <div className="page-stack grid min-w-0 max-w-full gap-6">
      <header className="page-heading">
        <div>
          <h1 className="m-0 text-2xl font-semibold">个人数据</h1>
          <p className="mb-0 mt-1 text-sm muted">统计记录时间 · {timezone} · 历史默认近7个自然日</p>
        </div>
      </header>

      {overview.error && <ErrorBanner message={overview.error} retry={overview.refresh} stale={!!overview.data} />}
      {overview.loading && !overview.data ? <Loading /> : overview.data && (
        <>
          <CoverageBanner coverage={overview.data.coverage} />
          <section className="panel">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="m-0 text-base font-semibold">今日额度与Token</h2>
                <p className="mb-0 mt-1 text-xs muted">订阅额度按各自账务窗口展示；今日Token按统计记录时间归日。</p>
              </div>
              <Zap aria-hidden="true" className="h-5 w-5 text-[var(--primary)]" />
            </div>
            <div className="mt-4 grid gap-4 xl:grid-cols-12">
              <TodayTokens tokens={overview.data.todayTokens} />
              <div className="grid grid-cols-[repeat(auto-fit,minmax(260px,1fr))] gap-3 xl:col-span-8">
                {overview.data.subscriptions.length ? overview.data.subscriptions.map((subscription) => (
                  <SubscriptionCard key={subscription.id} subscription={subscription} timezone={timezone} />
                )) : (
                  <div className="flex min-h-40 items-center gap-3 rounded-[10px] border border-dashed border-[var(--border-strong)] bg-[var(--surface-subtle)] p-5 col-span-full">
                    <span className="grid h-10 w-10 shrink-0 place-items-center rounded-[9px] bg-[var(--surface)] text-[var(--muted)]"><WalletCards aria-hidden="true" className="h-5 w-5" /></span>
                    <div><strong className="text-sm">无有效订阅</strong><p className="mb-0 mt-1 text-xs muted">今日Token仍按网关记录展示。</p></div>
                  </div>
                )}
              </div>
            </div>
          </section>
        </>
      )}

      <section className="panel min-w-0 max-w-full">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid h-9 w-9 place-items-center rounded-[9px] bg-[var(--primary-soft)] text-[var(--primary)]"><CalendarDays aria-hidden="true" className="h-4 w-4" /></span>
            <div>
              <h2 className="m-0 text-base font-semibold">年度Token热力图</h2>
              <p className="mb-0 mt-1 text-xs muted">全部模型；色阶跨保留年份一致。点击日期定位下方历史数据和日志。</p>
            </div>
          </div>
          <label className="grid gap-1 text-xs muted">
            年份
            <Select value={year} onChange={(event) => setYear(Number(event.target.value))}>
              {[0, 1, 2].map((offset) => <option key={offset} value={platformToday.year - offset}>{platformToday.year - offset}</option>)}
            </Select>
          </label>
        </div>
        {heatmap.error && <div className="mt-4"><ErrorBanner message={heatmap.error} retry={heatmap.refresh} stale={!!heatmap.data} /></div>}
        {heatmap.loading && !heatmap.data ? <Loading /> : heatmap.data ? (
          <>
            <div className="mt-4"><CoverageBanner coverage={heatmap.data.coverage} /></div>
            <Chart label="年度Token热力图" height={210} onEvents={{ click: selectDay }} option={buildHeatmapOption(heatmap.data.days, heatmap.data.thresholds, year)} />
            <div className="mt-2 flex flex-wrap justify-end gap-x-4 gap-y-2 text-xs muted" aria-label="热力图数据状态">
              <LegendSwatch className="border border-[var(--border)] bg-[var(--surface-subtle)]" label="零用量" />
              <LegendSwatch className="bg-[var(--border)]" label="未采集" />
              <LegendSwatch className="border border-dashed border-[var(--border-strong)]" label="未来" />
            </div>
          </>
        ) : heatmap.error ? null : <Empty />}
      </section>

      <section className="panel min-w-0 max-w-full">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="m-0 text-base font-semibold">历史分析</h2>
            <p className="mb-0 mt-1 text-xs muted">日均包含真实零消耗日并排除未来日期。</p>
          </div>
          <AnalyticsFilters value={filters} onChange={updateFilters} models={models} />
        </div>
        {analytics.error && <div className="mt-4"><ErrorBanner message={analytics.error} retry={analytics.refresh} stale={!!analytics.data} /></div>}
        {analytics.loading && !analytics.data ? <Loading /> : analytics.data && (
          <>
            <div className="mt-4"><CoverageBanner coverage={analytics.data.coverage} /></div>
            <div className="mt-4 grid overflow-hidden rounded-[10px] border border-[var(--border)] md:grid-cols-3 xl:grid-cols-6">
              <MetricCard label="活跃天数" value={analytics.data.activeDays} />
              <MetricCard label="总Token" value={analytics.data.totalTokens} />
              <MetricCard label="日均Token" value={analytics.data.averageDailyTokens} formula="总Token ÷ 所选自然日数" />
              <MetricCard label="请求次数" value={analytics.data.requests} detail="用量记录条数" />
              <MetricCard label="输出Token" value={analytics.data.outputTokens} />
              <MetricCard label="缓存命中率" value={analytics.data.cacheHitRate === null ? null : analytics.data.cacheHitRate * 100} unit="%" formula="Σ缓存读 ÷ (Σ输入 + Σ缓存写 + Σ缓存读)" />
            </div>
          </>
        )}
      </section>

      {analytics.data && (
        <div className="grid items-stretch gap-6 xl:grid-cols-12">
          <section className="panel xl:col-span-8">
            <div className="flex min-h-12 flex-wrap items-center justify-between gap-3">
              <div className="flex flex-wrap items-center gap-3">
                <h2 className="m-0 text-base font-semibold">用量趋势</h2>
                <Segmented value={metric} onChange={setMetric} label="时间指标" options={[
                  { value: "totalTokens", label: "总Token" },
                  { value: "outputTokens", label: "输出Token" },
                  { value: "cacheHitRate", label: "缓存命中率" },
                ]} />
              </div>
              <Segmented value={mode} onChange={setMode} label="图表模式" options={[
                { value: "line", label: "折线趋势" },
                { value: "bar", label: "柱状分布" },
              ]} />
            </div>
            <TimeSeriesChart data={analytics.data.series} metric={metric} mode={mode} percent={metric === "cacheHitRate"} label="个人历史时间趋势" height={350} />
          </section>
          <section className="panel xl:col-span-4">
            <h2 className="m-0 text-base font-semibold">模型调用分布</h2>
            <p className="mb-0 mt-1 text-xs muted">按请求次数排序。</p>
            <div className="mt-2">{analytics.data.models.length ? <ModelDonut models={analytics.data.models} compact /> : <Empty />}</div>
          </section>
        </div>
      )}

      <section className="panel min-w-0 max-w-full">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="m-0 text-base font-semibold">个人日志</h2>
            <p className="mb-0 mt-1 text-xs muted">时间倒序，与上方历史分析使用相同筛选。</p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Segmented value={tab} onChange={(value) => { setTab(value); setCursors([undefined]); }} label="日志类型" options={[
              { value: "usage", label: "用量日志" },
              { value: "errors", label: "错误日志" },
            ]} />
            <label className="flex items-center gap-2 text-xs muted">
              每页
              <Select value={pageSize} onChange={(event) => { setPageSize(Number(event.target.value)); setCursors([undefined]); }}>
                {[20, 50, 100].map((size) => <option key={size}>{size}</option>)}
              </Select>
            </label>
          </div>
        </div>
        {logs.error && <div className="mt-4"><ErrorBanner message={logs.error} retry={logs.refresh} stale={!!logs.data} /></div>}
        {logs.data && <div className="mt-4"><CoverageBanner coverage={logs.data.coverage} /></div>}
        {logs.loading && !logs.data ? <Loading /> : logs.data?.items.length ? (
          <div className="mt-3 min-w-0 max-w-full overflow-x-auto rounded-[10px] border border-[var(--border)]">
            <table className="w-full min-w-[1040px] text-sm">
              <thead className="bg-[var(--surface-subtle)] text-left muted">
                <tr>{tab === "usage" ? (
                  <><th className="pl-4">统计时间</th><th>部门</th><th>模型</th><th>API Key</th><th>Token 明细</th><th>性能</th></>
                ) : (
                  <><th className="pl-4">时间</th><th>模型</th><th>错误类型</th><th>简要原因</th></>
                )}<th className="pr-4 text-right">元数据</th></tr>
              </thead>
              <tbody>{logs.data.items.map((row: UsageLog | ErrorLog) => <LogRow key={row.id} row={row} kind={tab} timezone={timezone} />)}</tbody>
            </table>
          </div>
        ) : <Empty title="没有日志" detail={logs.data?.coverage.state === "uncollected" ? "该时间范围的日志尚未采集。" : "当前筛选范围没有可展示的日志。"} />}
        <div className="mt-4 flex items-center justify-between gap-3 border-t border-[var(--border)] pt-4">
          <span className="text-xs muted">第 {cursors.length} 页 · 每页 {pageSize} 条</span>
          <div className="flex gap-2">
            <Button variant="secondary" disabled={cursors.length <= 1} onClick={() => setCursors((items) => items.slice(0, -1))}>上一页</Button>
            <Button variant="secondary" disabled={!logs.data?.hasMore || !logs.data.nextCursor} onClick={() => logs.data?.nextCursor && setCursors((items) => [...items, logs.data!.nextCursor])}>下一页</Button>
            <Button variant="secondary" onClick={logs.refresh}>刷新日志</Button>
          </div>
        </div>
      </section>
    </div>
  );
}

function TodayTokens({ tokens }: { tokens: TokenBreakdown }) {
  const items = [
    ["普通输入", tokens.input], ["输出", tokens.output], ["缓存写入", tokens.cacheWrite], ["缓存读取", tokens.cacheRead],
  ] as const;
  return (
    <div className="relative overflow-hidden rounded-[10px] border border-[var(--border)] bg-[var(--primary-soft)] p-5 xl:col-span-4">
      <div className="text-xs font-semibold text-[var(--primary)]">今天总Token</div>
      <div className="mt-2 text-3xl font-semibold tabular-nums" title={tokens.total === null ? "无有效样本" : String(tokens.total)}>{formatMetric(tokens.total)}</div>
      <div className="mt-5 grid grid-cols-2 gap-x-5 gap-y-3">
        {items.map(([label, value]) => <div key={label} className="border-t border-[color-mix(in_srgb,var(--primary)_18%,transparent)] pt-2"><div className="text-xs muted">{label}</div><div className="mt-1 text-sm font-semibold tabular-nums">{formatMetric(value)}</div></div>)}
      </div>
    </div>
  );
}

function SubscriptionCard({ subscription, timezone }: { subscription: SubscriptionUsage; timezone: string }) {
  const progress = subscription.limit === null || subscription.limit <= 0 ? null : Math.min(100, (subscription.used / subscription.limit) * 100);
  const status = subscription.status === "unlimited" ? "不限额" : subscription.status === "exceeded" ? "已超限" : "有效";
  return (
    <article className="rounded-[10px] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]">
      <div className="flex items-start justify-between gap-3"><div className="min-w-0"><h3 className="m-0 truncate text-sm font-semibold" title={subscription.name}>{subscription.name}</h3><p className="mb-0 mt-1 text-xs muted">当前订阅额度</p></div><span className="rounded-full bg-[var(--surface-subtle)] px-2 py-1 text-xs font-semibold text-[var(--primary)]">{status}</span></div>
      <div className="mt-4 flex items-end gap-1.5"><strong className="text-2xl font-semibold tabular-nums">{money(subscription.used)}</strong><span className="pb-1 text-xs muted">{subscription.currency} 已用</span></div>
      {progress === null ? (
        <div className="mt-3 flex h-2 items-center rounded-full bg-[var(--surface-subtle)]"><div className="h-0.5 w-full bg-[var(--primary)] opacity-35" /></div>
      ) : (
        <div className="mt-3 h-2 overflow-hidden rounded-full bg-[var(--surface-subtle)]" role="progressbar" aria-label={subscription.name + "额度使用进度"} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(progress)}><div className={subscription.status === "exceeded" ? "h-full rounded-full bg-[var(--danger)]" : "h-full rounded-full bg-[var(--primary)]"} style={{ width: progress + "%" }} /></div>
      )}
      <dl className="mt-4 grid grid-cols-2 gap-3 text-xs">
        <div><dt className="muted">额度</dt><dd className="m-0 mt-1 font-semibold tabular-nums">{subscription.limit === null ? "不限" : money(subscription.limit) + " " + subscription.currency}</dd></div>
        <div><dt className="muted">剩余</dt><dd className="m-0 mt-1 font-semibold tabular-nums">{subscription.limit === null ? "不限" : subscription.remaining === null ? "—" : money(subscription.remaining) + " " + subscription.currency}</dd></div>
      </dl>
      <div className="mt-3 flex items-center gap-1.5 border-t border-[var(--border)] pt-3 text-xs muted"><Clock3 aria-hidden="true" className="h-3.5 w-3.5" />{subscription.resetsAt ? dateTime(subscription.resetsAt, timezone) + " 重置" : "未提供重置时间"}{subscription.status === "exceeded" && " · 已超额"}</div>
    </article>
  );
}

function LegendSwatch({ className, label }: { className: string; label: string }) {
  return <span className="inline-flex items-center gap-1.5"><span className={"h-3 w-3 rounded-sm " + className} aria-hidden="true" />{label}</span>;
}

function LogRow({ row, kind, timezone }: { row: UsageLog | ErrorLog; kind: "usage" | "errors"; timezone: string }) {
  const [open, setOpen] = useState(false);
  const toggle = <button className="log-detail-button inline-flex items-center gap-1 text-[var(--primary)]" aria-expanded={open} onClick={() => setOpen((value) => !value)}>{open ? <ChevronUp aria-hidden="true" className="h-3.5 w-3.5" /> : <ChevronDown aria-hidden="true" className="h-3.5 w-3.5" />}{open ? "收起" : "展开"}</button>;
  if (kind === "usage") {
    const usage = row as UsageLog;
    return <>
      <tr className="border-t border-[var(--border)] align-top">
        <td className="py-3 pl-4 whitespace-nowrap">{dateTime(usage.recordedAt, timezone)}</td><td>{usage.department || "未分配"}</td><td className="max-w-64 [overflow-wrap:anywhere]">{usage.model}</td><td className="max-w-48 [overflow-wrap:anywhere]">{usage.apiKeyName}</td>
        <td><div className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs"><LogValue label="输入" value={usage.inputTokens} /><LogValue label="输出" value={usage.outputTokens} /><LogValue label="缓存写" value={usage.cacheWriteTokens} /><LogValue label="缓存读" value={usage.cacheReadTokens} /></div></td>
        <td><div className="grid gap-1 text-xs"><LogValue label="耗时" value={usage.durationMs} unit="ms" /><LogValue label="TTFT" value={usage.ttftMs} unit="ms" /></div></td><td className="pr-4 text-right">{toggle}</td>
      </tr>
      {open && <tr><td colSpan={7} className="border-t border-[var(--border)] bg-[var(--surface-subtle)] px-4 py-3"><pre className="m-0 max-h-48 overflow-auto whitespace-pre-wrap break-words text-xs">{JSON.stringify(usage.metadata, null, 2)}</pre></td></tr>}
    </>;
  }
  const error = row as ErrorLog;
  return <>
    <tr className="border-t border-[var(--border)] align-top"><td className="py-3 pl-4 whitespace-nowrap">{dateTime(error.recordedAt, timezone)}</td><td>{error.model}</td><td>{error.type}</td><td className="max-w-xl whitespace-normal leading-5">{error.reason}</td><td className="pr-4 text-right">{toggle}</td></tr>
    {open && <tr><td colSpan={5} className="border-t border-[var(--border)] bg-[var(--surface-subtle)] px-4 py-3"><pre className="m-0 max-h-48 overflow-auto whitespace-pre-wrap break-words text-xs">{JSON.stringify(error.metadata, null, 2)}</pre></td></tr>}
  </>;
}

function LogValue({ label, value, unit = "" }: { label: string; value: number | null; unit?: string }) {
  return <span className="inline-flex min-w-0 items-baseline justify-between gap-2"><span className="muted">{label}</span><span className="tabular-nums">{value === null ? "—" : value.toLocaleString("zh-CN")}{value !== null && unit}</span></span>;
}
