/* eslint-disable react-refresh/only-export-components */
import type { EChartsOption } from "echarts";
import type { ChartMode, TimePoint } from "../../lib/types";
import { Chart, palette } from "./Chart";
import { cartesianTheme, chartColors, compactNumber, compactTimeLabel, fullNumber, tooltipTheme } from "./chartTheme";
import { percent as formatPercent } from "../../lib/format";

type TimeMetric = "totalTokens" | "outputTokens" | "cacheHitRate" | "successRate" | "users" | "requests";
const metricNames: Record<TimeMetric, string> = {
  totalTokens: "总 Token", outputTokens: "输出 Token", cacheHitRate: "缓存命中率",
  successRate: "请求成功率", users: "用户数", requests: "请求次数",
};

export function buildTimeSeriesOption(data: TimePoint[], metric: TimeMetric, mode: ChartMode, percent = false): EChartsOption {
  const values = data.map(point => {
    const value = point[metric];
    return typeof value === "number" ? (percent ? value * 100 : value) : null;
  });
  const colors = chartColors();
  const color = metric === "outputTokens" || metric === "successRate" ? palette[1] : palette[0];
  const theme = cartesianTheme(data.length > 16 ? 64 : 32);
  return {
    color: [color],
    textStyle: theme.textStyle,
    tooltip: {
      ...tooltipTheme(), trigger: "axis",
      axisPointer: { type: "line", lineStyle: { color, opacity: .3, type: "dashed" } },
      valueFormatter: value => value === null || value === undefined ? "—" : percent ? formatPercent(Number(value) / 100) : fullNumber(value),
    },
    grid: theme.grid,
    xAxis: {
      ...theme.xAxis, type: "category", boundaryGap: mode === "bar", data: data.map(point => point.bucket),
      axisLabel: { ...(theme.xAxis.axisLabel as object), formatter: compactTimeLabel, hideOverlap: true },
    },
    yAxis: {
      ...theme.yAxis, type: "value", min: 0, ...(percent ? { max: 100 } : {}),
      axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: percent ? "{value}%" : compactNumber },
    },
    series: [{
      name: metricNames[metric], type: mode, connectNulls: false,
      data: values.map((value, index) => data[index].incomplete ? { value, symbol: "emptyCircle", symbolSize: 7, itemStyle: { borderWidth: 2 } } : value),
      smooth: mode === "line" ? .16 : false,
      showSymbol: data.length <= 8 || data.some(point => point.incomplete), symbolSize: 5,
      lineStyle: { width: 2.5 },
      itemStyle: { borderRadius: mode === "bar" ? [5, 5, 0, 0] : 0 }, barMaxWidth: 30,
      areaStyle: mode === "line" ? { opacity: 1, color: { type: "linear", x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: color + "20" }, { offset: 1, color: color + "00" }] } } : undefined,
      emphasis: { focus: "series", lineStyle: { width: 3 } },
      markPoint: {
        symbol: "circle", symbolSize: 7,
        label: { show: true, formatter: "未结束", position: "top", distance: 8, fontSize: 11, color: colors.muted, backgroundColor: colors.surface, padding: [3, 6], borderRadius: 4 },
        data: data.flatMap((point, index) => point.incomplete && values[index] !== null ? [{ name: "未结束", coord: [point.bucket, values[index]] }] : []),
      },
    }] as EChartsOption["series"],
    dataZoom: data.length > 16 ? [
      { type: "inside", filterMode: "none" },
      { type: "slider", filterMode: "none", height: 14, bottom: 4, borderColor: "transparent", backgroundColor: colors.surfaceSubtle, fillerColor: color + "18", showDetail: false, brushSelect: false },
    ] : [],
  };
}

export function TimeSeriesChart({ data, metric, mode, label, percent = false, height = 280 }: {
  data: TimePoint[]; metric: TimeMetric; mode: ChartMode; label: string; percent?: boolean; height?: number;
}) {
  return <Chart label={label} height={height} option={buildTimeSeriesOption(data, metric, mode, percent)} />;
}
