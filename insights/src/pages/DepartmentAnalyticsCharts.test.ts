import { describe, expect, it } from "vitest";
import type { DepartmentAnalytics, TimePoint } from "../lib/types";
import { convertCost, cumulativeActualCost, groupedDepartments, heavyUsersForMonth, modelFamilyData, modelFamilyMonthlyData, pastCostTrend } from "./DepartmentAnalyticsCharts";

const series: TimePoint[] = [{
  bucket: "2026-07-01T00:00:00+08:00", totalTokens: 35, outputTokens: 3, cacheHitRate: null, requests: 2,
  models: [
    { modelId: "openai:gpt-6", name: "gpt-6", platform: "openai", requests: 1, totalTokens: 20, outputTokens: 2 },
    { modelId: "deepseek:deepseek-v4", name: "deepseek-v4", platform: "deepseek", requests: 1, totalTokens: 15, outputTokens: 1 },
    { modelId: "opencodego:deepseek-flash", name: "deepseek-flash", platform: "opencode", requests: 1, totalTokens: 5, outputTokens: 1 },
  ],
  members: [{ id: 7, name: "Ray", totalTokens: 21 }, { id: 8, name: "Lin", totalTokens: 14 }],
}, {
  bucket: "2026-08-01T00:00:00+08:00", totalTokens: 40, outputTokens: 4, cacheHitRate: null, requests: 2,
  models: [
    { modelId: "openai:gpt-6.1", name: "gpt-6.1", platform: "openai", requests: 1, totalTokens: 30, outputTokens: 3 },
    { modelId: "zai:glm-5", name: "glm-5", platform: "zai", requests: 1, totalTokens: 10, outputTokens: 1 },
  ],
  members: [{ id: 8, name: "Lin", totalTokens: 25 }, { id: 7, name: "Ray", totalTokens: 15 }],
}];

describe("department analytics chart data", () => {
  it("groups model tokens by family for all months and one selected month", () => {
    expect(modelFamilyData(series, "all")).toEqual([{ name: "OpenAI", value: 50 }, { name: "DeepSeek", value: 20 }, { name: "ZhipuGLM", value: 10 }]);
    expect(modelFamilyData(series, "2026-08")).toEqual([{ name: "OpenAI", value: 30 }, { name: "ZhipuGLM", value: 10 }]);
    expect(modelFamilyMonthlyData(series)).toEqual({
      months: ["2026-07", "2026-08"],
      families: [
        { name: "OpenAI", total: 50, values: [20, 30] },
        { name: "DeepSeek", total: 20, values: [20, 0] },
        { name: "ZhipuGLM", total: 10, values: [0, 10] },
      ],
    });
    const wrappedUnknown = [{ ...series[0], models: [{ modelId: "opencode_go:mystery", name: "mystery", platform: "opencode_go", requests: 1, totalTokens: 7, outputTokens: 0 }] }];
    expect(modelFamilyData(wrappedUnknown, "all")).toEqual([{ name: "其他", value: 7 }]);
  });

  it("uses real monthly members and the complete-range member list", () => {
    const extraJuly = { bucket: "2026-07-15T00:00:00+08:00", totalTokens: 4, outputTokens: 0, cacheHitRate: null, requests: 1, members: [{ id: 7, name: "Ray", totalTokens: 4 }] } satisfies TimePoint;
    const data = { series: [...series, extraJuly], memberPareto: [{ id: "7", name: "Ray", tokens: 40, cumulativeShare: 0.51 }, { id: "8", name: "Lin", tokens: 39, cumulativeShare: 1 }] } as DepartmentAnalytics;
    expect(heavyUsersForMonth(data, "all").map((item) => item.tokens)).toEqual([40, 39]);
    expect(heavyUsersForMonth(data, "2026-07")).toEqual([{ id: 7, name: "Ray", tokens: 25 }, { id: 8, name: "Lin", tokens: 14 }]);
    expect(heavyUsersForMonth(data, "2026-08")).toEqual([{ id: 8, name: "Lin", tokens: 25 }, { id: 7, name: "Ray", tokens: 15 }]);
  });

  it("groups long department tails and excludes the current cost month", () => {
    const departments = Array.from({ length: 9 }, (_, index) => ({ id: String(index), name: `部门${index}`, count: 9 - index }));
    expect(groupedDepartments(departments)).toHaveLength(8);
    expect(groupedDepartments(departments).at(-1)).toEqual({ name: "其他 2 个部门", value: 3 });
    expect(groupedDepartments([{ id: "__unassigned__", name: "未分配", count: 4 }, ...departments.slice(0, 6)])).toEqual([
      ...departments.slice(0, 6).map((item) => ({ name: item.name, value: item.count })),
      { name: "其他", value: 4 },
    ]);
    const past = pastCostTrend([{ month: "2026-09", actualCost: 12.5 }, { month: "2026-10", actualCost: 8.25 }, { month: "2026-11", actualCost: 99 }], "2026-11");
    expect(past).toEqual([{ month: "2026-09", actualCost: 12.5 }, { month: "2026-10", actualCost: 8.25 }]);
    expect(cumulativeActualCost(past)).toBe(20.75);
    expect(convertCost(20.75, "usd")).toBe(20.75);
    expect(convertCost(20.75, "cny")).toBeCloseTo(139.025);
  });

  it("limits heavy users to ten visible rows", () => {
    const memberPareto = Array.from({ length: 12 }, (_, index) => ({ id: String(index), name: `用户${index}`, tokens: 12 - index, cumulativeShare: 0 }));
    const data = { series: [], memberPareto } as unknown as DepartmentAnalytics;
    expect(heavyUsersForMonth(data, "all")).toHaveLength(10);
  });
});
