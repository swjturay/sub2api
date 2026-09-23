import type { EChartsOption } from "echarts";
import { metric } from "../../lib/format";

const fallback = {
  surface: "#ffffff",
  surfaceSubtle: "#f5f7fa",
  border: "#d9e0ea",
  ink: "#172033",
  muted: "#667085",
  primary: "#1769e0",
};

function cssColor(name: string, fallbackColor: string) {
  if (typeof document === "undefined") return fallbackColor;
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallbackColor;
}

export const chartColors = () => ({
  surface: cssColor("--surface", fallback.surface),
  surfaceSubtle: cssColor("--surface-subtle", fallback.surfaceSubtle),
  border: cssColor("--border", fallback.border),
  ink: cssColor("--ink", fallback.ink),
  muted: cssColor("--muted", fallback.muted),
  primary: cssColor("--primary", fallback.primary),
});

export const chartFontFamily = () => {
  if (typeof document === "undefined") return "system-ui, sans-serif";
  return getComputedStyle(document.documentElement).fontFamily || "system-ui, sans-serif";
};

export type ChartColors = ReturnType<typeof chartColors>;

export function recolorChartOption<T>(value: T, from: ChartColors, to: ChartColors): T {
  const replacements = new Map(Object.keys(from).map((key) => [from[key as keyof ChartColors], to[key as keyof ChartColors]]));
  const visit = (item: unknown): unknown => {
    if (typeof item === "string") return replacements.get(item) ?? item;
    if (Array.isArray(item)) return item.map(visit);
    if (item && typeof item === "object") {
      return Object.fromEntries(Object.entries(item).map(([key, nested]) => [key, visit(nested)]));
    }
    return item;
  };
  return visit(value) as T;
}

export const palette = [
  "#2878e6", "#13a37c", "#e49a22", "#8a63d2", "#d94f70",
  "#2396b8", "#708a2c", "#d06c2f", "#68758a",
];

export const heatPalette = ["#d9e9ff", "#96c2ff", "#4a91ee", "#1769cf"];

export function compactTimeLabel(value: unknown) {
  const raw = String(value ?? "");
  const match = raw.match(/^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2}))?/);
  if (!match) return raw;
  const [, , month, day, hour, minute] = match;
  return hour ? `${month}-${day} ${hour}:${minute}` : `${month}-${day}`;
}

export function compactNumber(value: unknown) {
  if (value === null || value === undefined) return "—";
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  return metric(number);
}

export function fullNumber(value: unknown) {
  if (value === null || value === undefined) return "—";
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  if (number !== 0 && Math.abs(number) < 1) return metric(number);
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 6 }).format(number);
}

export function cartesianTheme(bottom = 46): Pick<EChartsOption, "grid" | "textStyle"> & {
  xAxis: Record<string, unknown>;
  yAxis: Record<string, unknown>;
} {
  const colors = chartColors();
  const axis = {
    axisLine: { lineStyle: { color: colors.border } },
    axisTick: { show: false },
    axisLabel: { color: colors.muted, fontSize: 11, margin: 12 },
  };
  return {
    textStyle: { color: colors.ink, fontFamily: chartFontFamily() },
    grid: { left: 58, right: 20, top: 24, bottom, containLabel: true },
    xAxis: { ...axis, splitLine: { show: false } },
    yAxis: {
      ...axis,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: colors.border, opacity: 0.58 } },
    },
  };
}

export function tooltipTheme() {
  const colors = chartColors();
  return {
    confine: true,
    backgroundColor: colors.surface,
    borderColor: colors.border,
    borderWidth: 1,
    padding: [9, 11] as [number, number],
    textStyle: { color: colors.ink, fontSize: 12 },
    extraCssText: "max-width:min(320px,calc(100vw - 32px));white-space:normal;overflow-wrap:anywhere;border-radius:8px;box-shadow:0 4px 8px rgba(15,23,42,.10);",
  };
}

export function legendTheme() {
  const colors = chartColors();
  return {
    type: "scroll" as const,
    icon: "roundRect",
    itemWidth: 10,
    itemHeight: 6,
    itemGap: 16,
    textStyle: { color: colors.muted, fontSize: 11 },
    pageIconColor: colors.primary,
    pageIconInactiveColor: colors.border,
    pageTextStyle: { color: colors.muted },
  };
}
