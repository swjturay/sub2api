import { describe, expect, it } from "vitest";
import { adaptModelProfile } from "../../lib/api";
import type { ModelProfile } from "../../lib/types";
import { catalogPrice, emptyCatalogFilters, filterCatalog, modelVendor, supportsCapability } from "./modelCatalog";

const model = (platform: string, extra: Partial<ModelProfile> = {}) => ({ ...adaptModelProfile({ identity: { platform, name: "shared-name" }, profile: {}, performance: {} }), ...extra });

describe("model catalog discovery", () => {
  it("combines vendor, capability and modality filters without treating unknown as supported", () => {
    const models = [model("openai", { reasoning: "supported", toolCalling: "supported", inputModalities: ["image"], useCases: ["代码开发"] }), model("anthropic", { reasoning: "supported", toolCalling: "unknown", inputModalities: ["image"] })];
    expect(filterCatalog(models, { search: "  代码  ", vendor: "openai", capabilities: ["reasoning", "toolCalling"], input: "image" }).map(item => item.id)).toEqual(["openai:shared-name"]);
    expect(filterCatalog(models, { ...emptyCatalogFilters, vendor: "anthropic", capabilities: ["toolCalling"] })).toEqual([]);
    expect(filterCatalog(models, emptyCatalogFilters)).toHaveLength(2);
  });

  it("groups publishers independently from routing platforms without changing API identities", () => {
    const gemini = model("antigravity", { id: "antigravity:gemini-3.1-pro" });
    const claude = model("antigravity", { id: "antigravity:claude-sonnet-4-6" });
    const deepseek = model("opencode_go", { id: "opencode_go:deepseek-flash" });
    expect([gemini, claude, deepseek].map(modelVendor)).toEqual(["gemini", "anthropic", "deepseek"]);
    expect(filterCatalog([gemini, claude, deepseek], { ...emptyCatalogFilters, vendor: "gemini" })).toEqual([gemini]);
    expect(modelVendor(model("antigravity", { name: "Gemini-like unknown model" }))).toBe("unknown");
  });

  it("derives extra capabilities only from declared modalities and the explicit context threshold", () => {
    const multimodal = model("gemini", { inputModalities: ["text", "image", "audio", "video", "pdf"], outputModalities: ["text"], contextLimit: 128_000 });
    for (const key of ["vision", "audio", "video", "documents", "longContext"] as const) expect(supportsCapability(multimodal, key)).toBe(true);
    expect(supportsCapability(multimodal, "imageGeneration")).toBe(false);
    expect(supportsCapability({ ...multimodal, outputModalities: ["image"] }, "imageGeneration")).toBe(true);
    expect(supportsCapability({ ...multimodal, contextLimit: 127_999 }, "longContext")).toBe(false);
    expect(supportsCapability(model("a"), "longContext")).toBe(false);
    expect(filterCatalog([multimodal, model("a")], { ...emptyCatalogFilters, capabilities: ["audio", "video", "documents"] })).toEqual([multimodal]);
  });

  it("keeps missing measurements last, preserves true zero and never mutates the original catalog", () => {
    const unknown = model("a");
    const zero = model("b", { metrics: { ...unknown.metrics, ttft: { value: 0 } } });
    const measured = model("c", { metrics: { ...unknown.metrics, ttft: { value: 100 } } });
    const original = [unknown, measured, zero];
    expect(filterCatalog(original, emptyCatalogFilters, "latency").map(item => item.id)).toEqual([zero.id, measured.id, unknown.id]);
    expect(original).toEqual([unknown, measured, zero]);
  });

  it("does not present a conditional or tiered price as an unconditional quote", () => {
    expect(catalogPrice(model("a"), "input")).toBe("未配置");
    expect(catalogPrice(model("a", { pricing: [{ label: "input", value: "0 USD/token" }] }), "input")).toBe("0 USD/百万 Token");
    expect(catalogPrice(model("a", { pricing: [{ label: "input", value: "0.000003 USD/token", condition: "max_tokens=200000" }] }), "input")).toBe("分档 / 条件计价");
    expect(catalogPrice(model("a", { pricing: [{ label: "input", value: "3 USD/request" }] }), "input")).toBe("3 USD/request");
  });
});
