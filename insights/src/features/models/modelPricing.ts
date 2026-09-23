import type { ModelProfile } from "../../lib/types";

export type PricingEntry = ModelProfile["pricing"][number];

export interface PricingConditionPart {
  label: string;
  value: string;
}

export interface FormattedPricingEntry {
  label: string;
  displayValue: string;
  rawValue: string;
  rawCondition?: string;
  conditions: PricingConditionPart[];
}

const pricingLabels: Record<string, string> = {
  input: "输入",
  output: "输出",
  cache_read: "缓存读取",
  cache_write: "缓存写入",
};

const conditionLabels: Record<string, string> = {
  min_tokens: "起始 Token",
  max_tokens: "截止 Token",
  tier: "价格档位",
};

const usdPerToken = /^([+]?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?)\s+USD\/token$/i;

function formatMillionTokenPrice(rawValue: string) {
  const match = rawValue.trim().match(usdPerToken);
  if (!match) return rawValue;
  const perToken = Number(match[1]);
  if (!Number.isFinite(perToken) || perToken < 0) return rawValue;
  const perMillion = perToken * 1_000_000;
  if (!Number.isFinite(perMillion)) return rawValue;
  const formatted = new Intl.NumberFormat("zh-CN", {
    useGrouping: true,
    maximumSignificantDigits: 12,
  }).format(perMillion);
  return formatted + " USD/百万 Token";
}

function formatConditionValue(key: string, value: string) {
  if ((key === "min_tokens" || key === "max_tokens") && /^\d+$/.test(value)) {
    const parsed = Number(value);
    if (Number.isSafeInteger(parsed)) return parsed.toLocaleString("zh-CN");
  }
  return value;
}

export function formatPricingEntry(entry: PricingEntry): FormattedPricingEntry {
  const rawCondition = entry.condition?.trim() || undefined;
  const conditionParts = rawCondition?.split(",").map((part) => part.trim()).filter(Boolean) || [];
  const parsedConditions = conditionParts.map((part) => {
    const separator = part.indexOf("=");
    if (separator <= 0) return { label: "条件", value: part };
    const key = part.slice(0, separator).trim();
    const value = part.slice(separator + 1).trim();
    if (!conditionLabels[key]) return { label: "条件", value: part };
    return {
      label: conditionLabels[key],
      value: formatConditionValue(key, value),
    };
  });

  return {
    label: pricingLabels[entry.label] || entry.label || "参考价格",
    displayValue: formatMillionTokenPrice(entry.value),
    rawValue: entry.value,
    rawCondition,
    conditions: parsedConditions,
  };
}
