import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { TooltipProvider } from "../components/ui/Tooltip";
import { GatewayPage } from "./GatewayPage";

vi.mock("../features/BootstrapContext", () => ({ useBootstrap: () => ({ generatedAt: "2026-09-28T12:00:00+08:00", timezone: "Asia/Shanghai", models: [], departments: [] }) }));
vi.mock("../lib/useRemote", () => ({ useRemote: () => ({ data: {
  totalCalls: 10, failedCalls: 1, successRate: 0.9, modelDuration: { value: 1 }, gatewayDuration: { value: 1 }, series: [], totalUsers: 5, newUsers: 1,
  frequency: { high: 1, medium: 2, low: 1, unclassified: 1, coverage: { state: "partial", message: "历史采集不完整" } },
  userSeries: [], funnel: [], preferences: [], coverage: { state: "complete" }, retentionCoverage: { state: "complete" }, preferenceCoverage: { state: "complete" },
} }) }));
vi.mock("../features/Filters", () => ({ AnalyticsFilters: () => null }));
vi.mock("../components/charts/TimeSeriesChart", () => ({ TimeSeriesChart: () => null }));
vi.mock("../components/charts/Chart", () => ({ Chart: () => null }));
afterEach(cleanup);

it("removes the unclassified card but retains coverage warnings and accessible help", async () => {
  render(<TooltipProvider delayDuration={0}><GatewayPage auto={false} /></TooltipProvider>);
  expect(screen.queryByText("未分类用户")).not.toBeInTheDocument();
  expect(screen.getByText("低频用户")).toBeInTheDocument();
  expect(screen.getByRole("note")).toHaveAccessibleName(/历史采集不完整/);
  const explanation = "频次按整个所选区间累计；切换粒度不会重新分类。";
  expect(screen.queryByText(explanation)).not.toBeInTheDocument();
  const user = userEvent.setup();
  screen.getByRole("button", { name: "用户分析说明" }).focus();
  expect(await screen.findByRole("tooltip")).toHaveTextContent(explanation);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
});
