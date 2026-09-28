import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { TooltipProvider } from "../components/ui/Tooltip";
import { GatewayPage } from "./GatewayPage";

const timing = vi.hoisted(() => ({ model: 19000 as number | null, gateway: 1700.2 as number | null }));
vi.mock("../features/BootstrapContext", () => ({ useBootstrap: () => ({ generatedAt: "2026-09-28T12:00:00+08:00", timezone: "Asia/Shanghai", models: [], departments: [] }) }));
vi.mock("../lib/useRemote", () => ({ useRemote: () => ({ data: {
  totalCalls: 10, failedCalls: 1, successRate: 0.9, modelDuration: { value: timing.model }, gatewayDuration: { value: timing.gateway }, series: [], totalUsers: 5, newUsers: 1,
  frequency: { high: 1, medium: 2, low: 1, unclassified: 1, coverage: { state: "partial", message: "历史采集不完整" } },
  userSeries: [], funnel: [], preferences: [], coverage: { state: "complete" }, retentionCoverage: { state: "complete" }, preferenceCoverage: { state: "complete" },
} }) }));
vi.mock("../features/Filters", () => ({ AnalyticsFilters: () => null }));
vi.mock("../components/charts/TimeSeriesChart", () => ({ TimeSeriesChart: () => null }));
vi.mock("../components/charts/Chart", () => ({ Chart: () => null }));
afterEach(cleanup);

it.each([
  [19000, 1700.2, "19s", "1.7s"],
  [0, 15, "0s", "0.015s"],
  [null, null, "—", "—"],
])("renders millisecond API timings %s/%s in seconds without losing missing values", (model, gateway, expectedModel, expectedGateway) => {
  timing.model = model as number | null;
  timing.gateway = gateway as number | null;
  render(<TooltipProvider><GatewayPage auto={false} /></TooltipProvider>);
  const modelCard = screen.getByText("模型平均处理时长").closest(".metric-card");
  const gatewayCard = screen.getByText("网关平均转发前耗时").closest(".metric-card");
  expect(modelCard?.querySelector(".metric-card__value")?.textContent).toBe(expectedModel);
  expect(gatewayCard?.querySelector(".metric-card__value")?.textContent).toBe(expectedGateway);
});

it("explains the pre-forward boundary without claiming pure compute time", async () => {
  render(<TooltipProvider delayDuration={0}><GatewayPage auto={false} /></TooltipProvider>);
  screen.getByRole("button", { name: "网关平均转发前耗时计算方法" }).focus();
  const tooltip = await screen.findByRole("tooltip");
  expect(tooltip).toHaveTextContent("并发排队");
  expect(tooltip).toHaveTextContent("不是纯计算耗时");
  expect(tooltip).toHaveTextContent("不包含首次发送后的模型响应时间");
});

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
