import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { TooltipProvider } from "../components/ui/Tooltip";
import { CostPage } from "./CostPage";

vi.mock("../features/BootstrapContext", () => ({ useBootstrap: () => ({ generatedAt: "2026-09-28T12:00:00+08:00", timezone: "Asia/Shanghai" }) }));
vi.mock("../lib/useRemote", () => ({ useRemote: () => ({ data: {
  summary: { contributorCount: 1, accountCount: 3, platformCost: 30, actualCost: 20, savings: 10, completedAccounts: 1, totalAccounts: 2 },
  trend: [
    { month: "2026-07", platformCost: 10, actualCost: 0, savings: 0, completedAccounts: 0, totalAccounts: 2 },
    { month: "2026-08", platformCost: 30, actualCost: 0, savings: 30, completedAccounts: 1, totalAccounts: 2 },
  ],
  contributionDepartments: [{ id: "eng", name: "研发", contributorCount: 1, accountCount: 3, requestCount: 3, tokens: 10, platformCost: 30, actualCost: 20, savings: 10, completedAccounts: 1, totalAccounts: 2 }],
  usageDepartments: [], flows: [], accounts: { items: [], total: 0, page: 1, pages: 1, pageSize: 20 },
  dimensions: { departments: [], platforms: [], contributors: [], statuses: [], paymentMethods: [] }, coverage: { state: "complete" },
} }) }));
vi.mock("../components/charts/Chart", () => ({ Chart: ({ option, label }: { option: unknown; label: string }) => <div data-testid={label}>{JSON.stringify(option)}</div> }));

afterEach(cleanup);

it("shows only monetary series and a single monetary axis in the cost trend", () => {
  render(<TooltipProvider><CostPage auto={false} /></TooltipProvider>);
  const option = JSON.parse(screen.getByTestId("成本数据十二个月趋势").textContent!);
  expect(option.series.map((series: { name: string }) => series.name)).toEqual(["预估价格", "真实支出", "成本节省"]);
  expect(Array.isArray(option.yAxis) ? option.yAxis : [option.yAxis]).toHaveLength(1);
  expect(option.series[1].data).toEqual([null, 0]);
  expect(option.series[2].data).toEqual([null, 30]);
  expect(screen.getByText("1/2 个账号")).toBeInTheDocument();
  expect(screen.getByText("1/2")).toBeInTheDocument();
  expect(screen.getByText("50")).toBeInTheDocument();
});
