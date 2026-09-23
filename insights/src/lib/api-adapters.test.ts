import { describe, expect, it } from "vitest";
import { adaptModelSlice, adaptTimePoint, adaptTokens } from "./api";

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
