import { afterEach, describe, expect, it, vi } from "vitest";
import { adaptModelSlice, adaptTimePoint, adaptTokens, insightsApi, resolveCatalogModel } from "./api";

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
  it("maps model usage nested in a time bucket", () => {
    expect(adaptTimePoint({ start: "2026-09-22", metrics: { tokens: { total: 12 } }, models: [{ model: "openai:gpt-6", request_count: 2, tokens: { total: 12, output: 3 } }] })).toMatchObject({
      models: [{ modelId: "openai:gpt-6", name: "openai:gpt-6", requests: 2, totalTokens: 12, outputTokens: 3 }],
    });
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
  it("maps subscription minute usage without changing quota totals", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      code: 0,
      data: {
        data: { subscriptions: [{ id: 7, name: "daily", used_amount: 12, limit_amount: 20, remaining_amount: 8, currency: "USD", recent_usage: [{ at: "2026-09-24T02:01:00Z", amount: 0.02, requests: 4 }] }], tokens: {}, actual_cost: 1.25 },
        meta: { timezone: "Asia/Shanghai", generated_at: "2026-09-24T10:01:00+08:00", coverage: [] },
      },
    }), { status: 200 }));
    await expect(insightsApi.personalOverview()).resolves.toMatchObject({
      subscriptions: [{ id: "7", used: 12, recentUsage: [{ amount: 0.02, requests: 4 }], recentUsagePreview: false }],
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
  it("resolves an unknown usage provider from a unique catalog identity", () => {
    expect(resolveCatalogModel("unknown:gemini-3.8-flash-high", [
      { id: "antigravity:gemini-3.8-flash-high", name: "Gemini 3.8 Flash High", platform: "antigravity" },
      { id: "openai:gpt-6", name: "GPT-6", platform: "openai" },
    ])).toMatchObject({ id: "antigravity:gemini-3.8-flash-high", platform: "antigravity" });
  });
  it("keeps missing cost spend nullable while preserving confirmed zero", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      data: {
        month: "2026-09", in_progress: true,
        summary: { contributor_count: 1, account_count: 2, platform_cost: "10.00", actual_cost: "0.00", savings: "4.00", completed_accounts: 1, total_accounts: 2 },
        trend: [], contribution_departments: [], usage_departments: [], flows: [],
        accounts: { total: 2, page: 1, page_size: 20, pages: 1, items: [
          { id: 1, name: "missing", platform: "openai", type: "oauth", status: "active", registered: true, inherited: false, request_count: 0, tokens: {}, platform_cost: "6.00", actual_cost: null, savings: null },
          { id: 2, name: "zero", platform: "openai", type: "oauth", status: "active", registered: true, inherited: false, request_count: 0, tokens: {}, platform_cost: "4.00", actual_cost: "0.00", savings: "4.00" },
        ] },
        dimensions: { contributors: [], departments: [], platforms: [], statuses: [], payment_methods: [] },
      },
      meta: { timezone: "Asia/Tokyo", generated_at: "2026-09-24T10:00:00+09:00", coverage: [] },
    }), { status: 200 }));
    await expect(insightsApi.costs({ month: "2026-09", department: "", platform: "", q: "", contributor: "", paymentMethod: "", status: "", completeness: "", registration: "", page: 1, pageSize: 20 })).resolves.toMatchObject({
      accounts: { items: [{ actualCost: null, savings: null }, { actualCost: 0, savings: 4 }] },
    });
  });
});
