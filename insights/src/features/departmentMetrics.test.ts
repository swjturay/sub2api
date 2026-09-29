import { describe, expect, it } from "vitest";
import type { DepartmentAnalytics } from "../lib/types";
import { departmentMetricGroups } from "./departmentMetrics";

describe("department metric bands", () => {
  it("renders all nine confirmed metrics including output token quantity", () => {
    const data = {
      members: 6,
      activeMembers: 5,
      totalTokens: 58174,
      averageDailyTokens: 8310.57,
      perMemberDailyTokens: 1385.1,
      requests: 273,
      outputTokens: 11234,
      outputShare: 0.1931,
      cacheHitRate: 0.142,
    } as DepartmentAnalytics;

    const groups = departmentMetricGroups(data);
    expect(groups.map(group => group.title)).toEqual(["成员与使用", "Token规模", "消耗结构"]);
    expect(groups.map(group => [group.primary.label, ...group.secondary.map(item => item.label)])).toEqual([
      ["总成员数", "活跃成员数", "请求次数"],
      ["总Token", "日均Token", "每日人均Token"],
      ["输出Token", "输出Token占比", "缓存命中率"],
    ]);
    expect(groups.flatMap(group => [group.primary, ...group.secondary])).toContainEqual(
      expect.objectContaining({ label: "输出Token", value: 11234 }),
    );
    expect(groups[2].secondary).toContainEqual(
      expect.objectContaining({ label: "输出Token占比", value: 19.31 }),
    );
  });
});
