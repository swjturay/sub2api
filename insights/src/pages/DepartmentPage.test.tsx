import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { insightsApi } from "../lib/api";
import type { DepartmentAnalytics, DepartmentOverview } from "../lib/types";
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
  vi.spyOn(insightsApi, "departmentsOverview").mockImplementation(() => new Promise((resolve) => { resolveOverview = resolve; }));
  const details = vi.spyOn(insightsApi, "departments").mockResolvedValue({
    members: 2, activeMembers: 1, totalTokens: 10, averageDailyTokens: 10,
    perMemberDailyTokens: 5, requests: 1, outputTokens: 2, outputShare: 0.2,
    cacheHitRate: 0, series: [], models: [], departmentPareto: [], memberPareto: [],
    topUsers: [], performance: { tpm: { value: 1 }, rpm: { value: 1 }, ttft: { value: 1 }, tpot: { value: 1 } },
    coverage: { state: "complete" },
  } satisfies DepartmentAnalytics);

  render(<TooltipProvider><DepartmentPage auto={false} /></TooltipProvider>);
  expect(details).not.toHaveBeenCalled();
  await act(async () => resolveOverview({
    members: 2, activeMembers: 1, totalTokens: 10, averageDailyTokens: 10,
    perMemberDailyTokens: 5, requests: 1, outputTokens: 2, outputShare: 0.2,
    cacheHitRate: 0, series: [], coverage: { state: "complete" },
  }));
  expect(await screen.findByText("总成员数")).toBeInTheDocument();
  await waitFor(() => expect(details).toHaveBeenCalledTimes(1));
});
