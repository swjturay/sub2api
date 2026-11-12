import { afterEach, describe, expect, it, vi } from "vitest";
import { adaptModelSlice, adaptTimePoint, adaptTokens, coalesceDepartmentOverviewSeries, insightsApi, resolveCatalogModel } from "./api";

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

describe("Insights API adapters", () => {
  it("normalizes cost mutation responses for immediate row updates", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      code: 0,
      data: { saved: true, account_id: 7, month: "2026-09", contributor_user_id: 3, payment_method: "payg", actual_cost: "2.50", notes: "ok", updated_at: "2026-11-05T01:00:00Z" },
    }), { status: 200 }));
    await expect(insightsApi.saveCostMonth(7, "2026-09", { contributor_user_id: 3, payment_method: "payg", actual_cost: "2.50", notes: "ok" })).resolves.toMatchObject({ accountId: 7, month: "2026-09", actualCost: "2.50", updatedAt: "2026-11-05T01:00:00Z" });
  });
  it("bootstraps from lightweight model options without requesting full model analytics", async () => {
    localStorage.setItem("auth_user", JSON.stringify({ id: 7, role: "user", can_view_insights: true }));
    const requested: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requested.push(url);
      if (url.includes("/models/options")) return new Response(JSON.stringify({ data: { items: [{ platform: "openai", name: "gpt-6", display_name: "GPT-6" }] }, meta: { timezone: "Asia/Shanghai", generated_at: "2026-11-04T00:00:00Z", coverage: [] } }), { status: 200 });
      if (url.includes("/insights/dimensions")) return new Response(JSON.stringify({ data: { department: { options: [] }, retention: {} }, meta: { timezone: "Asia/Shanghai", generated_at: "2026-11-04T00:00:00Z", coverage: [] } }), { status: 200 });
      throw new Error(`unexpected request: ${url}`);
    });
    await expect(insightsApi.bootstrap()).resolves.toMatchObject({ models: [{ id: "openai:gpt-6", name: "GPT-6" }] });
    expect(requested).toContain("/api/v1/insights/models/options");
    expect(requested.some((url) => url.includes("models?window=24h"))).toBe(false);
    expect(requested).toContain("/api/v1/insights/dimensions?include_models=false");
  });
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
    expect(adaptTimePoint({ start: "2026-09-22", metrics: { tokens: { total: 12 } }, models: [{ model: "openai:gpt-6", request_count: 2, tokens: { total: 12, output: 3 } }], members: [{ user_id: 7, username: "Ray", total_tokens: 12 }] })).toMatchObject({
      models: [{ modelId: "openai:gpt-6", name: "openai:gpt-6", requests: 2, totalTokens: 12, outputTokens: 3 }],
      members: [{ id: 7, name: "Ray", totalTokens: 12 }],
    });
  });
  it("coalesces split monthly department overview buckets from mixed timezone identities", () => {
    const series = coalesceDepartmentOverviewSeries([
      adaptTimePoint({ start: "2026-11-01T00:00:00+08:00", tokens: { total: 116_810_000_000, output: 4_000_000_000 }, request_count: 80, complete: false }),
      adaptTimePoint({ start: "2026-11-01T00:00:00+08:00", tokens: { total: 6_620_000_000, output: 300_000_000 }, request_count: 5, complete: false }),
    ], "month");
    expect(series).toHaveLength(1);
    expect(series[0]).toMatchObject({ totalTokens: 123_430_000_000, outputTokens: 4_300_000_000, requests: 85, incomplete: true });
  });
  it("loads platform user growth without department or model filters", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      data: { buckets: [{ start: "2026-10-01T00:00:00+08:00", total_users: 151, complete: true }] },
      meta: { timezone: "Asia/Shanghai", generated_at: "2026-11-04T00:00:00+08:00", coverage: [] },
    }), { status: 200 }));
    await expect(insightsApi.platformUserGrowth("2026-07-01", "2026-11-04")).resolves.toMatchObject({ series: [{ users: 151 }] });
    expect(String(fetch.mock.calls[0][0])).toBe("/api/v1/insights/gateway/users?from=2026-07-01&to=2026-11-04&granularity=month");
  });
  it("maps the all-user department distribution from the overview", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      data: { summary: { member_count: 2, active_member_count: 1, tokens: { total: 12, output: 3 }, daily_average_tokens: 4, daily_per_member_tokens: 2, request_count: 1, output_ratio: 0.25, cache_hit_ratio: 0 }, buckets: [], member_distribution: [{ id: "A/B", label: "研发", count: 2 }] },
      meta: { timezone: "Asia/Shanghai", generated_at: "2026-11-04T00:00:00+08:00", coverage: [] },
    }), { status: 200 }));
    await expect(insightsApi.departmentsOverview({ from: "2026-07-01", to: "2026-11-04", granularity: "month", models: [], departments: [] })).resolves.toMatchObject({ memberDistribution: [{ id: "A/B", name: "研发", count: 2 }] });
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
  it.each([
    ["department_attribute", "configured", "complete"],
    ["department_attribute", "complete", "complete"],
    ["department_attribute", "not_configured", "uncollected"],
    ["department_attribute", "invalid", "unavailable"],
    ["department_attribute", "partial", "partial"],
    ["cost_data", "configured", "unavailable"],
  ])("maps cost coverage %s/%s to %s without losing nullable spend", async (dataset, status, state) => {
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
      meta: { timezone: "Asia/Tokyo", generated_at: "2026-09-24T10:00:00+09:00", coverage: [{ dataset, status }] },
    }), { status: 200 }));
    await expect(insightsApi.costs({ month: "2026-09", department: "", platform: "", q: "", contributor: "", paymentMethod: "", status: "", completeness: "", registration: "", page: 1, pageSize: 20 })).resolves.toMatchObject({
      accounts: { items: [{ actualCost: null, savings: null }, { actualCost: 0, savings: 4 }] },
      coverage: { state },
    });
  });
});
