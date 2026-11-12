import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { insightsApi } from "../lib/api";
import type { CostDashboard, DepartmentAnalytics, DepartmentOverview } from "../lib/types";
import { DepartmentPage } from "./DepartmentPage";
import { TooltipProvider } from "../components/ui/Tooltip";

vi.mock("../features/BootstrapContext", () => ({
  useBootstrap: () => ({
    scope: "viewer:7",
    generatedAt: "2026-11-04T12:00:00+08:00",
    timezone: "Asia/Shanghai",
    models: [],
    departments: [],
  }),
}));
vi.mock("../components/charts/Chart", () => ({
  Chart: ({ label }: { label: string }) => <div>{label}</div>,
  palette: ["#000", "#111", "#222"],
}));

afterEach(() => vi.restoreAllMocks());

it("renders the department overview before requesting secondary analysis", async () => {
  let resolveOverview!: (value: DepartmentOverview) => void;
  const overview = vi.spyOn(insightsApi, "departmentsOverview").mockImplementation(() => new Promise((resolve) => { resolveOverview = resolve; }));
  vi.spyOn(insightsApi, "platformUserGrowth").mockResolvedValue({ series: [], coverage: { state: "complete" } });
  const costs = vi.spyOn(insightsApi, "costs").mockResolvedValue({
    month: "2026-11", inProgress: true,
    summary: { contributorCount: 0, accountCount: 0, platformCost: 0, actualCost: 0, savings: 0, completedAccounts: 0, totalAccounts: 0 },
    trend: [], contributionDepartments: [], usageDepartments: [], flows: [],
    accounts: { items: [], total: 0, page: 1, pageSize: 20, pages: 1 },
    dimensions: { contributors: [], departments: [], platforms: [], statuses: [], paymentMethods: [] },
    coverage: { state: "complete" },
  } satisfies CostDashboard);
  const details = vi.spyOn(insightsApi, "departments").mockResolvedValue({
    members: 2, activeMembers: 1, totalTokens: 10, averageDailyTokens: 10,
    perMemberDailyTokens: 5, requests: 1, outputTokens: 2, outputShare: 0.2,
    cacheHitRate: 0, series: [], models: [], departmentPareto: [], memberPareto: [],
    topUsers: [], performance: { tpm: { value: 1 }, rpm: { value: 1 }, ttft: { value: 1 }, tpot: { value: 1 } },
    coverage: { state: "complete" },
  } satisfies DepartmentAnalytics);

  render(<TooltipProvider><DepartmentPage auto={false} /></TooltipProvider>);
  expect(screen.queryByRole("heading", { name: "部门数据" })).not.toBeInTheDocument();
  expect(screen.queryByText("按用户当前单一部门归集历史；停用成员保留，已删除成员排除。")).not.toBeInTheDocument();
  expect(overview).toHaveBeenCalledWith({
    from: "2026-07-01",
    to: "2026-11-04",
    granularity: "month",
    models: [],
    departments: [],
  }, expect.any(AbortSignal));
  expect(details).not.toHaveBeenCalled();
  expect(costs).toHaveBeenCalledTimes(1);
  await act(async () => resolveOverview({
    members: 2, activeMembers: 1, totalTokens: 10, averageDailyTokens: 10,
    perMemberDailyTokens: 5, requests: 1, outputTokens: 2, outputShare: 0.2,
    cacheHitRate: 0, series: [], memberDistribution: [], coverage: { state: "partial", message: "历史汇总尚未完整采集" },
  }));
  expect(await screen.findByText("总成员数")).toBeInTheDocument();
  expect(screen.queryByRole("note", { name: /部分覆盖/ })).not.toBeInTheDocument();
  await waitFor(() => expect(details).toHaveBeenCalledTimes(1));
  for (const label of ["用户数说明", "用户部门分布说明", "研发部整体Token消耗说明", "模型使用比例说明", "重度用户TOP10说明", "部门模型性能说明"]) {
    expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
  }
  await waitFor(() => expect(screen.getByRole("button", { name: "人民币" })).toHaveAttribute("aria-pressed", "true"));
  expect(Array.from(screen.getByRole("group", { name: "支出成本币种" }).querySelectorAll("button"), (button) => button.textContent)).toEqual(["人民币", "美元"]);
  expect(screen.getByRole("button", { name: "人民币汇率说明" })).toBeInTheDocument();
  expect(screen.queryByText(/人民币按固定汇率/, { selector: "p" })).not.toBeInTheDocument();
});
