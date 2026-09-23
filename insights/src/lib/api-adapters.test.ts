import { afterEach, describe, expect, it, vi } from "vitest";
import { adaptModelSlice, adaptTimePoint, adaptTokens, insightsApi } from "./api";

afterEach(() => vi.restoreAllMocks());

describe("Insights API adapters", () => {
  it("preserves missing values as null while retaining observed zero", () => {
    expect(adaptTokens({ input: 0, output: null })).toEqual({
      input: 0,
      cacheWrite: null,
      cacheRead: null,
      output: null,
      total: null,
    });
    expect(
      adaptTimePoint({
        start: "2026-09-22",
        metrics: { request_count: 0, tokens: { total: null } },
      }),
    ).toMatchObject({ totalTokens: null, outputTokens: null, requests: 0 });
  });
  it("reads gateway success rate from the quality object and marks incomplete buckets", () => {
    expect(adaptTimePoint({ start: "2026-09-22T22:00:00+08:00", quality: { success_rate: 0.75 }, complete: false })).toMatchObject({ successRate: 0.75, incomplete: true });
  });
  it("maps today's actual cost independently from subscription quota", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      code: 0,
      data: {
        data: { subscriptions: [], tokens: { input: 10, output: 2, total: 12 }, actual_cost: 1.25 },
        meta: { timezone: "Asia/Shanghai", generated_at: "2026-09-23T12:00:00+08:00", coverage: [] },
      },
    }), { status: 200 }));
    await expect(insightsApi.personalOverview()).resolves.toMatchObject({
      totalAmount: 1.25,
      todayTokens: { input: 10, output: 2, total: 12 },
    });
  });
  it("builds a stable platform:name identity from a typed model object", () => {
    expect(
      adaptModelSlice({
        model: { platform: "openai", name: "gpt/test" },
        request_count: 0,
        tokens: { total: null, output: 0 },
      }),
    ).toMatchObject({
      modelId: "openai:gpt/test",
      name: "gpt/test",
      platform: "openai",
      requests: 0,
      totalTokens: null,
      outputTokens: 0,
    });
  });
});
