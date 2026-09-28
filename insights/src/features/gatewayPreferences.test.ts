import { describe, expect, it } from "vitest";
import { resolveGatewayPreferences } from "./gatewayPreferences";

describe("gateway model identities", () => {
  const catalog = [{ id: "openai:gpt-test", name: "GPT Test", platform: "openai" }];
  it("merges historical routing identities without changing department totals", () => {
    const result = resolveGatewayPreferences([{ department: "研发", total: 10, models: [
      { model: "composite:gpt-test", count: 3, share: 0.3 },
      { model: "openai:gpt-test", count: 5, share: 0.5 },
      { model: "unknown:gpt-test", count: 2, share: 0.2 },
    ] }], catalog);
    expect(result[0]).toEqual({ department: "研发", total: 10, models: [{ model: "openai:gpt-test", count: 10, share: 1 }] });
  });
  it("keeps ambiguous or missing identities unknown rather than guessing", () => {
    const result = resolveGatewayPreferences([{ department: "研发", total: 3, models: [
      { model: "composite:gpt-test", count: 1, share: 1 / 3 },
      { model: "composite:", count: 2, share: 2 / 3 },
    ] }], [...catalog, { id: "other:gpt-test", name: "Other", platform: "other" }]);
    expect(result[0].models.map((m) => m.model)).toEqual(["unknown:gpt-test", "unknown:unknown"]);
    expect(result[0].models.reduce((n, m) => n + m.count, 0)).toBe(3);
  });
});
