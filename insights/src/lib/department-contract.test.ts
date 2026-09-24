import { afterEach, describe, expect, it, vi } from "vitest";
import { insightsApi } from "./api";
import type { FilterState } from "./types";
const meta={timezone:"Asia/Shanghai",generated_at:"2026-09-22T00:00:00Z",coverage:[]};
const filters:FilterState={from:"2026-09-16",to:"2026-09-22",granularity:"day",models:[],departments:[]};
afterEach(()=>vi.restoreAllMocks());
describe("department Pareto contract",()=>{
 it("adapts complete department and member lists without a server mode parameter",async()=>{vi.spyOn(globalThis,"fetch").mockResolvedValue(new Response(JSON.stringify({data:{summary:{member_count:2,active_member_count:2,tokens:{total:30,output:3},daily_average_tokens:5,daily_per_member_tokens:2.5,request_count:2,output_ratio:.1,cache_hit_ratio:.2},buckets:[],models:[],pareto:{department_items:[{id:"d",label:"研发",total_tokens:30,cumulative_ratio:1}],member_items:[{id:"1",label:"甲",total_tokens:20,cumulative_ratio:2/3},{id:"2",label:"乙",total_tokens:10,cumulative_ratio:1}]},top_users:[],performance:{ttft_samples:4,tpot_samples:3}},meta}),{status:200}));const result=await insightsApi.departments(filters,"openai:gpt-4o");expect(result.departmentPareto).toHaveLength(1);expect(result.memberPareto).toHaveLength(2);expect(result.memberPareto.at(-1)?.cumulativeShare).toBe(1);expect(result.performance.ttft.sampleCount).toBe(4);expect(String(vi.mocked(fetch).mock.calls[0][0])).not.toContain("pareto_mode")});
 it("rejects a response missing either full Pareto list",async()=>{vi.spyOn(globalThis,"fetch").mockResolvedValue(new Response(JSON.stringify({data:{summary:{tokens:{}},buckets:[],models:[],pareto:{items:[]}},meta}),{status:200}));await expect(insightsApi.departments(filters,"")).rejects.toThrow("部门帕累托响应缺少完整部门或成员列表")});
});

it("does not require department Pareto fields on personal analytics", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: { summary: { active_days: 1, tokens: { total: 10, output: 2 }, daily_average_tokens: 10, request_count: 1, cache_hit_ratio: null }, buckets: [], models: [] }, meta }), { status: 200 }));
  await expect(insightsApi.personalAnalytics(filters)).resolves.toMatchObject({ totalTokens: 10, requests: 1 });
});

it("explains a partial usage window without exposing an internal boundary", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
    data: { summary: { active_days: 0, tokens: { total: 0, output: 0 }, daily_average_tokens: 0, request_count: 0, cache_hit_ratio: null }, buckets: [], models: [] },
    meta: { ...meta, coverage: [{ dataset: "usage_detail", status: "partial" }] },
  }), { status: 200 }));

  await expect(insightsApi.personalAnalytics(filters)).resolves.toMatchObject({
    coverage: {
      state: "partial",
      message: "用量历史尚未完整采集",
    },
  });
});
