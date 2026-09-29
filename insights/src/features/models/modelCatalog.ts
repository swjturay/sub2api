import type { ModelProfile } from "../../lib/types";
import { formatPricingEntry } from "./modelPricing";

export const capabilityOptions = [
  { value: "reasoning", label: "推理" },
  { value: "toolCalling", label: "工具调用" },
  { value: "structuredOutput", label: "结构化输出" },
  { value: "vision", label: "图像理解" },
  { value: "audio", label: "音频理解" },
  { value: "video", label: "视频理解" },
  { value: "documents", label: "文档输入" },
  { value: "imageGeneration", label: "图像生成" },
  { value: "longContext", label: "长上下文 ≥128K" },
] as const;
export type CatalogCapability = typeof capabilityOptions[number]["value"];
export type CatalogSort = "name" | "rpm" | "context" | "latency";
export interface CatalogFilters { search: string; vendor: string; capabilities: CatalogCapability[]; input: string }
export const emptyCatalogFilters: CatalogFilters = { search: "", vendor: "", capabilities: [], input: "" };

const vendors: Record<string, string> = { openai: "OpenAI", anthropic: "Anthropic", gemini: "Gemini", deepseek: "DeepSeek", grok: "xAI", kimi: "Moonshot AI", zhipu: "智谱", minimax: "MiniMax", unknown: "未归类" };
const modalities: Record<string, string> = { text: "文本", image: "图像", audio: "音频", video: "视频", file: "文件", pdf: "PDF" };
export const vendorLabel = (value: string) => vendors[value] || vendors.unknown;
export const modalityLabel = (value: string) => modalities[value] || value;
type CatalogIdentity = Pick<ModelProfile, "id" | "name" | "platform">;
export const modelIdentifier = (model: CatalogIdentity) => model.id.startsWith(`${model.platform}:`) ? model.id.slice(model.platform.length + 1) : model.id;
export const tokenLimit = (value: number | null) => value === null ? "未知" : new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 2 }).format(value);

// Catalog model families identify publishers; routing platforms remain API identities.
export function modelVendor(model: CatalogIdentity) {
  const name = modelIdentifier(model).toLowerCase();
  const families: Array<[RegExp, string]> = [
    [/^gemini(?:-|$)/, "gemini"], [/^claude(?:-|$)/, "anthropic"],
    [/^(?:gpt|chatgpt)(?:-|$)|^o\d(?:-|$)/, "openai"],
    [/^deepseek(?:-|$)/, "deepseek"], [/^glm(?:-|$)/, "zhipu"],
    [/^grok(?:-|$)/, "grok"], [/^kimi(?:-|$)/, "kimi"], [/^minimax(?:-|$)/, "minimax"],
  ];
  return families.find(([pattern]) => pattern.test(name))?.[1] ?? (vendors[model.platform] ? model.platform : "unknown");
}

export function supportsCapability(model: ModelProfile, capability: CatalogCapability) {
  switch (capability) {
    case "vision": return model.inputModalities.includes("image");
    case "audio": return model.inputModalities.includes("audio");
    case "video": return model.inputModalities.includes("video");
    case "documents": return model.inputModalities.some(value => value === "file" || value === "pdf");
    case "imageGeneration": return model.outputModalities.includes("image");
    case "longContext": return model.contextLimit !== null && model.contextLimit >= 128_000;
    default: return model[capability] === "supported";
  }
}

export function filterCatalog(models: ModelProfile[], filters: CatalogFilters, sort: CatalogSort = "name") {
  const query = filters.search.trim().toLocaleLowerCase();
  return models.filter(model => (!filters.vendor || modelVendor(model) === filters.vendor)
    && (!filters.input || model.inputModalities.includes(filters.input))
    && filters.capabilities.every(key => supportsCapability(model, key))
    && (!query || [model.id, model.name, vendorLabel(modelVendor(model)), model.description || "", ...model.useCases].join(" ").toLocaleLowerCase().includes(query)))
    .sort((a, b) => {
      if (sort !== "name") {
        const first = sort === "context" ? a.contextLimit : sort === "rpm" ? a.metrics.rpm.value : a.metrics.ttft.value;
        const second = sort === "context" ? b.contextLimit : sort === "rpm" ? b.metrics.rpm.value : b.metrics.ttft.value;
        if (first === null && second !== null) return 1;
        if (second === null && first !== null) return -1;
        if (first !== null && second !== null && first !== second) return sort === "latency" ? first - second : second - first;
      }
      return a.name.localeCompare(b.name, "zh-CN", { numeric: true }) || a.id.localeCompare(b.id);
    });
}

export function catalogPrice(model: ModelProfile, kind: "input" | "output") {
  const entries = model.pricing.filter(entry => entry.label === kind);
  if (!entries.length) return "未配置";
  if (entries.length > 1 || entries.some(entry => entry.condition?.trim())) return "分档 / 条件计价";
  return formatPricingEntry(entries[0]).displayValue;
}
