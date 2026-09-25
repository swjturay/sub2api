import { useDeferredValue, useMemo, useState, type ReactNode } from "react";
import type { EChartsOption } from "echarts";
import { Building2, Check, ChevronsUpDown, CircleDollarSign, Pencil, Search, SquareArrowOutUpRight, WalletCards } from "lucide-react";
import { Chart } from "../components/charts/Chart";
import { cartesianTheme, chartColors, compactNumber, legendTheme, tooltipTheme } from "../components/charts/chartTheme";
import { Button } from "../components/ui/Button";
import { Input, Select } from "../components/ui/Controls";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../components/ui/Dialog";
import { HelpTip } from "../components/ui/HelpTip";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "../components/ui/Popover";
import { CoverageBanner, Empty, ErrorBanner, Loading } from "../components/ui/States";
import { useBootstrap } from "../features/BootstrapContext";
import { costCompletionLabel, costPaymentLabels, costSavingsPreview } from "../features/costs";
import { insightsApi } from "../lib/api";
import { cn } from "../lib/cn";
import { dateTime, platformDateParts } from "../lib/format";
import type { CostAccount, CostFilters, CostPaymentMethod } from "../lib/types";
import { useRemote } from "../lib/useRemote";

const moneyFormatter = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 2, maximumFractionDigits: 2 });
const amountPattern = /^\d+(?:\.\d{1,2})?$/;

export function CostPage({ auto }: { auto: boolean }) {
  const boot = useBootstrap();
  const currentMonth = platformDateParts(boot.generatedAt, boot.timezone).date.slice(0, 7);
  const [filters, setFilters] = useState<CostFilters>({
    month: currentMonth,
    department: "",
    platform: "",
    q: "",
    contributor: "",
    paymentMethod: "",
    status: "",
    completeness: "",
    registration: "",
    page: 1,
    pageSize: 20,
  });
  const deferredSearch = useDeferredValue(filters.q);
  const deferredContributor = useDeferredValue(filters.contributor);
  const requestFilters = { ...filters, q: deferredSearch, contributor: deferredContributor };
  const state = useRemote((signal) => insightsApi.costs(requestFilters, signal), [
    filters.month, filters.department, filters.platform, deferredSearch, deferredContributor,
    filters.paymentMethod, filters.status, filters.completeness, filters.registration, filters.page, filters.pageSize,
  ], auto);
  const [editing, setEditing] = useState<CostAccount | null>(null);
  const [contributorID, setContributorID] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<CostPaymentMethod>("subscription");
  const [actualCost, setActualCost] = useState("");
  const [notes, setNotes] = useState("");
  const [mutationError, setMutationError] = useState("");
  const [mutating, setMutating] = useState(false);
  const data = state.data;
  const setFilter = <K extends keyof CostFilters>(key: K, value: CostFilters[K]) => setFilters((current) => ({ ...current, [key]: value, page: key === "page" ? Number(value) : 1 }));

  const openEditor = (account: CostAccount) => {
    setEditing(account);
    setContributorID(account.contributor ? String(account.contributor.id) : "");
    setPaymentMethod(account.paymentMethod || "subscription");
    setActualCost(account.actualCost === null ? "" : account.actualCost.toFixed(2));
    setNotes(account.notes);
    setMutationError("");
  };

  const save = async () => {
    if (!editing || !contributorID) {
      setMutationError("请选择贡献人。");
      return;
    }
    const normalized = actualCost.trim();
    if (normalized !== "" && (!amountPattern.test(normalized) || Number(normalized) < 0)) {
      setMutationError("真实支出必须是非负USD金额，最多保留两位小数。");
      return;
    }
    setMutating(true);
    setMutationError("");
    try {
      await insightsApi.saveCostMonth(editing.id, filters.month, {
        contributor_user_id: Number(contributorID),
        payment_method: paymentMethod,
        actual_cost: normalized || null,
        notes,
      });
      setEditing(null);
      await state.refresh();
    } catch (error) {
      setMutationError((error as Error).message);
    } finally {
      setMutating(false);
    }
  };

  const stopAfterMonth = async () => {
    if (!editing || !window.confirm(`确认让账号“${editing.name}”在 ${filters.month} 之后停止计入成本数据？`)) return;
    setMutating(true);
    setMutationError("");
    try {
      await insightsApi.stopCostAccount(editing.id, filters.month);
      setEditing(null);
      await state.refresh();
    } catch (error) {
      setMutationError((error as Error).message);
    } finally {
      setMutating(false);
    }
  };

  const trendOption = useMemo(() => data ? buildCostTrendOption(data.trend) : {}, [data]);
  const flowOption = useMemo(() => data ? buildCostFlowOption(data.flows) : {}, [data]);
  const previewSavings = editing ? costSavingsPreview(editing, actualCost) : null;
  const editingDeleted = Boolean(editing?.deletedAt && editing.deletedAt.slice(0, 7) < filters.month);

  return (
    <div className="cost-page page-stack grid gap-6">
      <header className="page-heading">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="m-0 text-2xl font-semibold">成本数据</h1>
            {data?.inProgress && <span className="cost-badge cost-badge--progress">本月进行中</span>}
          </div>
          <p className="mb-0 mt-1 text-sm muted">共享AI账号的月度真实支出、预估价格与成本节省，仅限内部管理。</p>
        </div>
      </header>

      <section className="filter-toolbar">
        <div className="analytics-filters" aria-label="成本数据筛选条件">
          <div className="filter-field"><span className="filter-label sr-only">月份</span><Input aria-label="月份" type="month" min="2026-06" max={currentMonth} value={filters.month} onChange={(event) => setFilter("month", event.target.value)} className="filter-trigger !h-[34px] w-40" /></div>
          <div className="filter-field"><span className="filter-label sr-only">贡献部门</span><SingleFilter label="贡献部门" value={filters.department} onChange={(value) => setFilter("department", value)} options={[{ value: "", label: "全部部门" }, ...(data?.dimensions.departments.map((department) => ({ value: department.id, label: department.name })) || [])]} className="w-44" /></div>
          <div className="filter-field"><span className="filter-label sr-only">平台</span><SingleFilter label="平台" value={filters.platform} onChange={(value) => setFilter("platform", value)} options={[{ value: "", label: "全部平台" }, ...(data?.dimensions.platforms.map((platform) => ({ value: platform, label: platform })) || [])]} className="w-40" /></div>
        </div>
        {(state.error || mutationError) && <div className="mt-3"><ErrorBanner message={mutationError || state.error || "请求失败"} retry={state.refresh} stale={!!data} /></div>}
      </section>

      {state.loading && !data ? <Loading /> : data && (
        <>
          <CoverageBanner coverage={data.coverage} />
          <section className="cost-metric-strip metric-strip grid md:grid-cols-3 xl:grid-cols-6" aria-label="月度成本概览">
            <CountMetric label="贡献人数" value={data.summary.contributorCount} detail="按当前部门归集" />
            <CountMetric label="贡献账号" value={data.summary.accountCount} detail="仅已登记账号" />
            <MoneyMetric label={<EstimatedPriceLabel />} value={data.summary.platformCost} detail="标准定价 x 用量口径" />
            <MoneyMetric label="已录入真实支出" value={data.summary.actualCost} detail="未录入账号不按零处理" />
            <MoneyMetric label="成本节省" value={data.summary.savings} detail="仅汇总已录入范围" emphasis />
            <CountMetric label="支出完整度" value={data.summary.totalAccounts ? data.summary.completedAccounts / data.summary.totalAccounts * 100 : 0} unit="%" detail={`${costCompletionLabel(data.summary.completedAccounts, data.summary.totalAccounts)} 个账号`} />
          </section>

          <section className="panel">
            <SectionHeading icon={<CircleDollarSign />} title="十二个月成本趋势" detail="真实支出与节省只统计已明确录入或确认0.00的账号。" />
            <Chart option={trendOption} label="成本数据十二个月趋势" height={320} />
          </section>

          <div className="grid items-stretch gap-6 xl:grid-cols-12">
            <section className="panel overflow-hidden xl:col-span-8">
              <SectionHeading icon={<WalletCards />} title="贡献部门" detail="真实支出和成本节省全部归属贡献人的当前部门。" />
              <div className="mt-4 overflow-auto scrollbar-thin">
                {data.contributionDepartments.length ? <table className="cost-table min-w-[850px] w-full"><thead><tr><th>部门</th><th>贡献人</th><th>账号</th><th>请求</th><th>Token</th><th><EstimatedPriceLabel /></th><th>真实支出</th><th>成本节省</th><th>完整度</th></tr></thead><tbody>{data.contributionDepartments.map((row) => <tr key={row.id}><td className="font-semibold">{row.name}</td><td>{row.contributorCount}</td><td>{row.accountCount}</td><td>{integer(row.requestCount)}</td><td>{integer(row.tokens)}</td><td>{money(row.platformCost)}</td><td>{money(row.actualCost)}</td><td className={moneyClass(row.savings)}>{money(row.savings)}</td><td>{costCompletionLabel(row.completedAccounts, row.totalAccounts)}</td></tr>)}</tbody></table> : <Empty />}
              </div>
            </section>
            <section className="panel overflow-hidden xl:col-span-4">
              <SectionHeading icon={<Building2 />} title="使用部门" detail="仅按调用用户当前部门归集请求、Token和预估价格。" />
              <div className="mt-4 overflow-auto scrollbar-thin">
                {data.usageDepartments.length ? <table className="cost-table min-w-[420px] w-full"><thead><tr><th>部门</th><th>请求</th><th>Token</th><th><EstimatedPriceLabel /></th></tr></thead><tbody>{data.usageDepartments.map((row) => <tr key={row.id}><td className="font-semibold">{row.name}</td><td>{integer(row.requestCount)}</td><td>{integer(row.tokens)}</td><td>{money(row.platformCost)}</td></tr>)}</tbody></table> : <Empty />}
              </div>
            </section>
          </div>

          <section className="panel">
            <SectionHeading icon={<SquareArrowOutUpRight />} title="部门成本流向" detail="贡献部门到使用部门的预估价格交叉汇总，不表示部门结算。" />
            {data.flows.length ? <Chart option={flowOption} label="贡献部门到使用部门成本流向矩阵" height={360} /> : <Empty />}
          </section>

          <section className="panel overflow-hidden !p-0">
            <div className="border-b border-[var(--border)] p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div><h2 className="m-0 text-base font-semibold">账号月表</h2><p className="mb-0 mt-1 text-xs muted">未登记账号可在此发现并配置；关联子账号已归入逻辑主账号。</p></div>
                <div className="text-xs muted">共 {data.accounts.total} 个账号</div>
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <label className="relative"><Search aria-hidden="true" className="absolute left-3 top-2.5 h-4 w-4 text-[var(--muted)]" /><Input value={filters.q} onChange={(event) => setFilter("q", event.target.value)} placeholder="账号名称或ID" className="w-52 pl-9" /></label>
                <Input value={filters.contributor} onChange={(event) => setFilter("contributor", event.target.value)} placeholder="贡献人" className="w-44" />
                <Select aria-label="付费方式筛选" value={filters.paymentMethod} onChange={(event) => setFilter("paymentMethod", event.target.value)}><option value="">全部付费方式</option>{data.dimensions.paymentMethods.map((method) => <option key={method.id} value={method.id}>{method.name}</option>)}</Select>
                <Select aria-label="账号状态筛选" value={filters.status} onChange={(event) => setFilter("status", event.target.value)}><option value="">全部状态</option>{data.dimensions.statuses.map((status) => <option key={status} value={status}>{costStatusLabel(status)}</option>)}</Select>
                <Select aria-label="真实支出完整性筛选" value={filters.completeness} onChange={(event) => setFilter("completeness", event.target.value)}><option value="">全部录入状态</option><option value="complete">已录入</option><option value="missing">未录入</option></Select>
                <Select aria-label="登记状态筛选" value={filters.registration} onChange={(event) => setFilter("registration", event.target.value)}><option value="">全部登记状态</option><option value="registered">已登记</option><option value="unregistered">未配置</option></Select>
              </div>
            </div>
            <div className="max-h-[620px] overflow-auto scrollbar-thin">
              <table className="cost-table cost-account-table min-w-[1740px] w-full">
                <thead className="sticky top-0 z-10 bg-[var(--surface)]"><tr><th>账号</th><th>平台/类型</th><th>状态</th><th>贡献人</th><th>当前部门</th><th>付费方式</th><th>请求</th><th>Token</th><th><EstimatedPriceLabel /></th><th>真实支出</th><th>成本节省</th><th>备注</th><th>最后修改</th><th><span className="sr-only">操作</span></th></tr></thead>
                <tbody>{data.accounts.items.map((account) => {
                  const cannotEdit = Boolean(account.deletedAt && account.deletedAt.slice(0, 7) < filters.month);
                  return <tr key={account.id}>
                    <td><div className="font-semibold text-[var(--ink)]">{account.name}</div><div className="mt-0.5 text-xs muted">ID {account.id}{account.inherited ? ` · 继承 ${account.configurationMonth}` : ""}</div></td>
                    <td><div>{account.platform}</div><div className="text-xs muted">{account.type}</div></td>
                    <td><span className={cn("cost-badge", account.deletedAt && "cost-badge--danger")}>{account.deletedAt ? "已删除" : account.status}</span>{account.expiresAt && <div className="mt-1 text-xs muted" title={dateTime(account.expiresAt, boot.timezone)}>到期 {platformDateParts(account.expiresAt, boot.timezone).date}</div>}</td>
                    <td>{account.contributor ? <><div>{account.contributor.name}</div><div className="text-xs muted">{account.contributor.email}</div></> : <span className="muted">未配置</span>}</td>
                    <td>{account.contributor?.department || "—"}</td>
                    <td>{account.paymentMethod ? costPaymentLabels[account.paymentMethod] : "—"}</td>
                    <td>{integer(account.requestCount)}</td><td>{integer(account.tokens.total || 0)}</td><td>{money(account.platformCost)}</td>
                    <td>{account.actualCost === null ? <span className="cost-missing">未录入</span> : money(account.actualCost)}</td>
                    <td className={moneyClass(account.savings)}>{account.savings === null ? "—" : money(account.savings)}</td>
                    <td className="max-w-56 truncate" title={account.notes}>{account.notes || "—"}</td>
                    <td>{account.updatedAt ? <><div>{account.updatedBy?.name || "管理员"}</div><div className="text-xs muted">{dateTime(account.updatedAt, boot.timezone)}</div></> : "—"}</td>
                    <td><Button type="button" variant="ghost" className="icon-button" onClick={() => openEditor(account)} disabled={cannotEdit} aria-label={`编辑 ${account.name}`} title={cannotEdit ? "软删除账号不能新增后续月份" : "编辑月记录"}><Pencil /></Button></td>
                  </tr>;
                })}</tbody>
              </table>
            </div>
            <div className="flex items-center justify-between border-t border-[var(--border)] px-4 py-3">
              <div className="flex items-center gap-2 text-xs muted"><span>每页</span><Select aria-label="每页数量" value={String(filters.pageSize)} onChange={(event) => setFilter("pageSize", Number(event.target.value))}><option value="20">20</option><option value="50">50</option><option value="100">100</option></Select></div>
              <div className="flex items-center gap-2"><Button variant="secondary" disabled={data.accounts.page <= 1} onClick={() => setFilter("page", data.accounts.page - 1)}>上一页</Button><span className="min-w-20 text-center text-xs muted">{data.accounts.page} / {data.accounts.pages}</span><Button variant="secondary" disabled={data.accounts.page >= data.accounts.pages} onClick={() => setFilter("page", data.accounts.page + 1)}>下一页</Button></div>
            </div>
          </section>
        </>
      )}

      <Dialog open={Boolean(editing)} onOpenChange={(open) => { if (!open && !mutating) setEditing(null); }}>
        <DialogContent className="max-w-2xl">
          <DialogHeader><DialogTitle>{editing?.registered ? "编辑账号月记录" : "登记贡献账号"}</DialogTitle><DialogDescription>{editing ? `${editing.name} · ${editing.platform} · ${filters.month}` : ""}</DialogDescription></DialogHeader>
          {editing && <div className="mt-5 grid gap-4">
            <div className="grid grid-cols-3 gap-3 rounded-md border border-[var(--border)] bg-[var(--surface-subtle)] p-3 text-sm"><div><span className="flex items-center text-xs muted"><EstimatedPriceLabel /></span><strong className="tabular-nums">{money(editing.platformCost)}</strong></div><div><span className="block text-xs muted">真实支出</span><strong className="tabular-nums">{actualCost.trim() ? money(Number(actualCost)) : "未录入"}</strong></div><div><span className="block text-xs muted">成本节省</span><strong className={cn("tabular-nums", moneyClass(previewSavings))}>{previewSavings === null ? "—" : money(previewSavings)}</strong></div></div>
            <div className="grid gap-4 md:grid-cols-2">
              <label className="field-label">贡献人<Select value={contributorID} onChange={(event) => setContributorID(event.target.value)} disabled={editingDeleted} className="w-full"><option value="">请选择现有用户</option>{data?.dimensions.contributors.map((person) => <option key={person.id} value={person.id}>{person.name} · {person.department}</option>)}</Select></label>
              <label className="field-label">付费方式<Select value={paymentMethod} onChange={(event) => setPaymentMethod(event.target.value as CostPaymentMethod)} disabled={editingDeleted} className="w-full"><option value="subscription">订阅</option><option value="payg">即用即付</option><option value="other">其他</option></Select></label>
            </div>
            <label className="field-label">真实支出（USD）<Input value={actualCost} onChange={(event) => setActualCost(event.target.value)} inputMode="decimal" placeholder="留空表示尚未录入；0.00表示确认无支出" disabled={editingDeleted} /></label>
            <label className="field-label">备注<textarea value={notes} onChange={(event) => setNotes(event.target.value)} maxLength={1000} rows={4} disabled={editingDeleted} className="cost-textarea" /></label>
            {editing.inherited && <p className="m-0 text-xs muted">贡献人和付费方式继承自 {editing.configurationMonth}；保存后会形成 {filters.month} 的明确月记录，真实支出不会自动继承。</p>}
            {editingDeleted && <p className="m-0 text-sm text-[var(--danger)]">该账号已软删除，不能新增删除月份之后的记录。</p>}
            {mutationError && <ErrorBanner message={mutationError} retry={() => void save()} />}
          </div>}
          <DialogFooter className="justify-between">
            <div>{editing?.registered && !editingDeleted && <Button type="button" variant="danger" onClick={stopAfterMonth} disabled={mutating}>本月后停止贡献</Button>}</div>
            <div className="flex gap-2"><Button type="button" variant="ghost" onClick={() => setEditing(null)} disabled={mutating}>取消</Button><Button type="button" onClick={save} disabled={mutating || editingDeleted}>{mutating ? "保存中" : "保存"}</Button></div>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function CountMetric({ label, value, unit, detail }: { label: string; value: number; unit?: string; detail: string }) {
  return <div className="metric-card"><div className="metric-card__label">{label}</div><div className="metric-card__value tabular-nums">{value.toLocaleString("zh-CN", { maximumFractionDigits: unit === "%" ? 1 : 0 })}{unit && <span className="metric-card__unit">{unit}</span>}</div><div className="metric-card__detail">{detail}</div></div>;
}

function SingleFilter({ label, value, onChange, options, className }: { label: string; value: string; onChange: (value: string) => void; options: Array<{ value: string; label: string }>; className?: string }) {
  const selected = options.find((option) => option.value === value) || options[0];
  return <Popover><PopoverTrigger asChild><Button type="button" variant="secondary" role="combobox" aria-label={label} className={cn("filter-trigger justify-between font-medium", className)}><span className="truncate">{selected?.label || label}</span><ChevronsUpDown aria-hidden="true" className="text-[var(--muted)]" /></Button></PopoverTrigger><PopoverContent align="start" role="listbox" aria-label={`${label}选项`} className="min-w-[var(--radix-popover-trigger-width)] p-1.5">{options.map((option) => <PopoverClose asChild key={option.value || "all"}><button type="button" role="option" aria-selected={option.value === value} onClick={() => onChange(option.value)} className="flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm outline-none hover:bg-[var(--surface-subtle)] focus-visible:bg-[var(--surface-subtle)]"><span className="grid size-4 shrink-0 place-items-center">{option.value === value && <Check aria-hidden="true" />}</span><span className="truncate">{option.label}</span></button></PopoverClose>)}</PopoverContent></Popover>;
}

function EstimatedPriceLabel() {
  return <span className="inline-flex items-center gap-0.5 whitespace-nowrap">预估价格<HelpTip label="预估价格计算方法">通过模型的标准定价x用量计算出来的。</HelpTip></span>;
}

function MoneyMetric({ label, value, detail, emphasis }: { label: ReactNode; value: number; detail: string; emphasis?: boolean }) {
  return <div className={cn("metric-card", emphasis && "cost-metric--emphasis")}><div className="metric-card__label">{label}</div><div className={cn("metric-card__value tabular-nums", emphasis && moneyClass(value))}>{money(value)}</div><div className="metric-card__detail">{detail}</div></div>;
}

function SectionHeading({ icon, title, detail }: { icon: ReactNode; title: string; detail: string }) {
  return <div className="flex items-start gap-3"><span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-[var(--secondary-soft)] text-[var(--secondary)] [&_svg]:h-4 [&_svg]:w-4">{icon}</span><div><h2 className="m-0 text-base font-semibold">{title}</h2><p className="mb-0 mt-1 text-xs muted">{detail}</p></div></div>;
}

function money(value: number) { return moneyFormatter.format(Number.isFinite(value) ? value : 0); }
function integer(value: number) { return Number(value || 0).toLocaleString("zh-CN"); }
function moneyClass(value: number | null) { return value !== null && value < 0 ? "text-[var(--danger)]" : value !== null && value > 0 ? "text-[var(--positive)]" : ""; }
function costStatusLabel(value: string) { return value === "deleted" ? "已删除" : value; }
function escapeHtml(value: unknown) { return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;"); }

function buildCostTrendOption(data: Array<{ month: string; platformCost: number; actualCost: number; savings: number; completedAccounts: number; totalAccounts: number }>): EChartsOption {
  const theme = cartesianTheme(58);
  return {
    color: ["#4f46e5", "#0f766e", "#d97706", "#64748b"],
    textStyle: theme.textStyle,
    tooltip: { ...tooltipTheme(), trigger: "axis", valueFormatter: (value) => typeof value === "number" ? money(value) : String(value ?? "—") },
    legend: { ...legendTheme(), bottom: 0 },
    grid: { ...(theme.grid as object), right: 52, bottom: 62 },
    xAxis: { ...theme.xAxis, type: "category", data: data.map((point) => point.month) },
    yAxis: [
      { ...theme.yAxis, type: "value", axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: (value: number) => `$${compactNumber(value)}` } },
      { ...theme.yAxis, type: "value", min: 0, max: 100, axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: "{value}%" } },
    ],
    series: [
      { name: "预估价格", type: "bar", barMaxWidth: 28, data: data.map((point) => point.platformCost), itemStyle: { borderRadius: [4, 4, 1, 1] } },
      { name: "真实支出", type: "line", connectNulls: false, smooth: 0.16, showSymbol: data.length <= 8, data: data.map((point) => point.completedAccounts ? point.actualCost : null) },
      { name: "成本节省", type: "line", connectNulls: false, smooth: 0.16, showSymbol: data.length <= 8, data: data.map((point) => point.completedAccounts ? point.savings : null) },
      { name: "录入完整度", type: "line", yAxisIndex: 1, symbol: "circle", symbolSize: 5, lineStyle: { type: "dashed", width: 1.5 }, data: data.map((point) => point.totalAccounts ? point.completedAccounts / point.totalAccounts * 100 : 0) },
    ],
  };
}

function buildCostFlowOption(flows: Array<{ contributionDepartment: string; usageDepartment: string; platformCost: number }>): EChartsOption {
  const contribution = [...new Set(flows.map((flow) => flow.contributionDepartment))];
  const usage = [...new Set(flows.map((flow) => flow.usageDepartment))];
  const colors = chartColors();
  const maximum = Math.max(0, ...flows.map((flow) => flow.platformCost));
  return {
    tooltip: { ...tooltipTheme(), position: "top", formatter: (params: unknown) => { const value = (params as { data: { value: [number, number, number] } }).data.value; return `<b>${escapeHtml(contribution[value[1]])} → ${escapeHtml(usage[value[0]])}</b><br/>${money(value[2])}`; } },
    grid: { left: 96, right: 42, top: 28, bottom: 104 },
    xAxis: { type: "category", data: usage, splitArea: { show: true }, axisLabel: { color: colors.muted, rotate: usage.length > 6 ? 28 : 0, width: 100, overflow: "truncate" } },
    yAxis: { type: "category", data: contribution, splitArea: { show: true }, axisLabel: { color: colors.muted, width: 88, overflow: "truncate" } },
    visualMap: { min: 0, max: maximum || 1, calculable: true, orient: "horizontal", left: "center", bottom: 0, inRange: { color: [colors.surfaceSubtle, "#99f6e4", "#0f766e"] }, textStyle: { color: colors.muted } },
    series: [{ name: "预估价格", type: "heatmap", data: flows.map((flow) => ({ value: [usage.indexOf(flow.usageDepartment), contribution.indexOf(flow.contributionDepartment), flow.platformCost] })), label: { show: contribution.length * usage.length <= 36, color: colors.ink, formatter: (params: unknown) => money((params as { data: { value: [number, number, number] } }).data.value[2]) }, emphasis: { itemStyle: { shadowBlur: 8, shadowColor: "rgba(15,23,42,.18)" } } }],
  };
}
