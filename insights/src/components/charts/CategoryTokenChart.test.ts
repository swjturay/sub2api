import { describe, expect, it } from "vitest";
import { adaptTimePoint } from "../../lib/api";
import { buildCategoryTokenOption, categoryTokenSeries } from "./CategoryTokenChart";

describe("classified token consumption", () => {
  it("keeps independent department and model distributions in each time bucket", () => {
    const points = [adaptTimePoint({ start: "2026-09-28", tokens: { total: 100, output: 20 },
      departments: [{ id: "a", label: "研发", tokens: { total: 80, output: 5 } }, { id: "b", label: "销售", tokens: { total: 20, output: 15 } }],
      models: [{ model: "openai:x", tokens: { total: 40, output: 18 } }, { model: "anthropic:x", tokens: { total: 60, output: 2 } }],
    }), adaptTimePoint({ start: "2026-09-29", tokens: { total: 7, output: 1 }, departments: [{ id: "b", label: "销售", tokens: { total: 7, output: 1 } }], models: [{ model: "openai:x", tokens: { total: 7, output: 1 } }] })];
    expect(categoryTokenSeries(points, "departments", "totalTokens").map(x => [x.id, x.values])).toEqual([["a", [80, 0]], ["b", [20, 7]]]);
    expect(categoryTokenSeries(points, "models", "outputTokens").map(x => [x.id, x.values])).toEqual([["openai:x", [18, 1]], ["anthropic:x", [2, 0]]]);
    for (const mode of ["line", "bar"] as const) {
      const option = buildCategoryTokenOption(points, "models", "totalTokens", mode);
      expect(option.series).toHaveLength(2);
      expect(option.series).toEqual(expect.arrayContaining([expect.objectContaining({ type: mode, data: [60, 0], stack: mode === "bar" ? "tokens" : undefined })]));
    }
  });

  it("preserves unavailable breakdowns and unknown token values instead of showing zero", () => {
    const points = [adaptTimePoint({ start: "1", departments: [{ id: "a", label: "研发", tokens: { total: 5 } }] }), adaptTimePoint({ start: "2" }), adaptTimePoint({ start: "3", departments: [{ id: "a", label: "研发", tokens: { total: null } }] }), adaptTimePoint({ start: "4", departments: [] })];
    expect(categoryTokenSeries(points, "departments", "totalTokens")[0].values).toEqual([5, null, null, 0]);
  });

  it("conserves every bucket total when grouping long tails, and keeps equal labels distinct", () => {
    const points = [adaptTimePoint({ start: "1", departments: Array.from({ length: 10 }, (_, i) => ({ id: String(i), label: "同名部门", tokens: { total: i + 1 } })) })];
    const groups = categoryTokenSeries(points, "departments", "totalTokens");
    expect(groups).toHaveLength(8);
    expect(new Set(groups.map(x => x.name)).size).toBe(8);
    expect(groups.at(-1)).toMatchObject({ name: "其他 3 个部门", values: [6] });
    expect(groups.reduce((sum, x) => sum + x.values[0]!, 0)).toBe(55);
  });

  it("formats monthly axis labels as an unambiguous Chinese year and month", () => {
    const points = [
      adaptTimePoint({ start: "2026-07-01T00:00:00+08:00", complete: true, departments: [] }),
      adaptTimePoint({ start: "2026-08-01T00:00:00+08:00", complete: false, departments: [] }),
    ];
    const option = buildCategoryTokenOption(points, "departments", "totalTokens", "bar", "month");
    const xAxis = option.xAxis as { axisLabel?: { formatter?: (value: string, index: number) => string } };
    const formatter = xAxis.axisLabel?.formatter;
    expect(formatter).toBeTypeOf("function");
    expect(formatter?.(points[0].bucket, 0)).toBe("2026年07月");
    expect(formatter?.(points[1].bucket, 1)).toBe("2026年08月\n未结束");
  });

  it("renders a directly labelled total series without department breakdown", () => {
    const points = [
      adaptTimePoint({ start: "2026-07-01", tokens: { total: 60_000_000_000, output: 2_000_000_000 }, departments: [{ id: "a", label: "研发", tokens: { total: 40_000_000_000 } }] }),
      adaptTimePoint({ start: "2026-08-01", tokens: { total: 150_000_000_000, output: 3_000_000_000 }, departments: [{ id: "b", label: "销售", tokens: { total: 150_000_000_000 } }] }),
    ];
    const option = buildCategoryTokenOption(points, "departments", "totalTokens", "bar", "month", true);
    const series = option.series as Array<{ name: string; data: Array<number | null>; stack?: string; label?: { show?: boolean; formatter?: (params: { value?: unknown }) => string } }>;
    const legend = option.legend as { show?: boolean };
    expect(legend.show).toBe(false);
    expect(series).toHaveLength(1);
    expect(series[0]).toMatchObject({ name: "总Token", data: [60_000_000_000, 150_000_000_000], stack: undefined, label: { show: true } });
    expect(series[0].label?.formatter?.({ value: 60_000_000_000 })).toBe("600亿");
  });
});
