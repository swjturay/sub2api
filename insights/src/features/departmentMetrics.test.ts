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
    expect(groups.primary).toHaveLength(3);
    expect(groups.secondary).toHaveLength(6);
    expect([...groups.primary, ...groups.secondary]).toHaveLength(9);
    expect(groups.secondary).toContainEqual(
      expect.objectContaining({ label: "输出Token", value: 11234 }),
    );
    expect(groups.secondary).toContainEqual(
      expect.objectContaining({ label: "输出Token占比", value: 19.31 }),
    );
  });
});
