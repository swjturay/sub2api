import type { EChartsOption } from "echarts";
import { metric } from "../../lib/format";

const fallback = {
  surface: "#ffffff",
  surfaceSubtle: "#f6f7fb",
  border: "#e3e7ef",
  ink: "#172033",
  muted: "#667085",
  primary: "#4f46e5",
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
  "#6366f1", "#0f9888", "#d69b36", "#a17cd4", "#d36d89",
  "#3299bd", "#819447", "#c8854b", "#7c889e",
];

export const heatPalette = ["#e0e7ff", "#a5b4fc", "#818cf8", "#4f46e5"];

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
    axisLabel: { color: colors.muted, fontSize: 12, margin: 12 },
  };
  return {
    textStyle: { color: colors.ink, fontFamily: chartFontFamily() },
    grid: { left: 10, right: 18, top: 28, bottom, containLabel: true },
    xAxis: { ...axis, splitLine: { show: false } },
    yAxis: {
      ...axis,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: colors.border, opacity: 0.5 } },
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
    extraCssText: "max-width:min(320px,calc(100vw - 32px));white-space:normal;overflow-wrap:anywhere;border-radius:10px;box-shadow:0 8px 28px rgba(15,23,42,.12);",
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
    textStyle: { color: colors.muted, fontSize: 12 },
    pageIconColor: colors.primary,
    pageIconInactiveColor: colors.border,
    pageTextStyle: { color: colors.muted },
  };
}
