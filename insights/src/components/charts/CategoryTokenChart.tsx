/* eslint-disable react-refresh/only-export-components */
import { useState } from "react";
import type { EChartsOption } from "echarts";
import type { ChartMode, Granularity, TimePoint } from "../../lib/types";
import { Chart, palette } from "./Chart";
import { cartesianTheme, chartColors, compactNumber, compactTimeLabel, fullNumber, legendTheme, tooltipTheme } from "./chartTheme";
import { Segmented } from "../ui/Controls";
import { Empty } from "../ui/States";
import { HelpTip } from "../ui/HelpTip";

type Dimension = "departments" | "models";
type TokenMetric = "totalTokens" | "outputTokens";

export function categoryTokenSeries(data: TimePoint[], dimension: Dimension, metric: TokenMetric) {
  const buckets = data.map(point => dimension === "departments" ? point.departments : point.models?.map(model => ({
    ...model, id: model.modelId,
    name: model.name === model.modelId && model.modelId.includes(":") ? model.name.slice(model.name.indexOf(":") + 1) : model.name,
  })));
  const categories = new Map<string, { id: string; name: string; total: number }>();
  for (const bucket of buckets) for (const item of bucket ?? []) {
    const current = categories.get(item.id) ?? { id: item.id, name: item.name, total: 0 };
    current.total += item[metric] ?? 0;
    categories.set(item.id, current);
  }
  const ranked = [...categories.values()].sort((a, b) => b.total - a.total || a.id.localeCompare(b.id));
  const names = new Map<string, number>();
  for (const item of ranked) names.set(item.name, (names.get(item.name) ?? 0) + 1);
  const groups = ranked.slice(0, 7).map(item => ({
    id: item.id, ids: [item.id], name: names.get(item.name)! > 1 ? `${item.name} (${item.id})` : item.name,
  }));
  if (ranked.length > 7) groups.push({ id: "__other_series__", ids: ranked.slice(7).map(item => item.id), name: `其他 ${ranked.length - 7} 个${dimension === "departments" ? "部门" : "模型"}` });
  return groups.map(group => ({ ...group, values: buckets.map(bucket => {
    // Missing breakdown is unknown; a missing category in a supplied bucket is zero.
    if (!bucket) return null;
    let sum = 0;
    for (const id of group.ids) {
      const item = bucket.find(item => item.id === id);
      if (item && item[metric] === null) return null;
      sum += item?.[metric] ?? 0;
    }
    return sum;
  }) }));
}

function categoryTimeLabel(value: string, granularity: Granularity) {
  if (granularity === "month") {
    const match = value.match(/^(\d{4})-(\d{2})/);
    if (match) return `${match[1]}年${match[2]}月`;
  }
  return compactTimeLabel(value);
}

export function buildCategoryTokenOption(data: TimePoint[], dimension: Dimension, metric: TokenMetric, mode: ChartMode, granularity: Granularity = "day", aggregate = false): EChartsOption {
  const metricLabel = metric === "totalTokens" ? "总Token" : "输出Token";
  const groups = aggregate ? [{
    id: "__total_series__", ids: [], name: metricLabel, values: data.map(point => point[metric]),
  }] : categoryTokenSeries(data, dimension, metric);
  const zoom = data.length > 16;
  const theme = cartesianTheme(zoom ? 90 : 62);
  const colors = chartColors();
  return {
    color: palette, textStyle: theme.textStyle, grid: theme.grid,
    legend: aggregate ? { show: false } : { ...legendTheme(), bottom: zoom ? 26 : 0 },
    tooltip: { ...tooltipTheme(), trigger: "axis", axisPointer: { type: mode === "bar" ? "shadow" : "line" }, valueFormatter: value => value == null ? "—" : fullNumber(value) },
    xAxis: { ...theme.xAxis, type: "category", boundaryGap: mode === "bar", data: data.map(point => point.bucket), axisLabel: { ...(theme.xAxis.axisLabel as object), hideOverlap: true, formatter: (value: string, index: number) => `${categoryTimeLabel(value, granularity)}${data[index]?.incomplete ? "\n未结束" : ""}` } },
    yAxis: { ...theme.yAxis, type: "value", min: 0, axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: compactNumber } },
    series: groups.map(group => ({
      id: group.id, name: group.name, type: mode, stack: !aggregate && mode === "bar" ? "tokens" : undefined,
      data: group.values, connectNulls: false, showSymbol: data.length <= 8, symbolSize: 5,
      lineStyle: { width: 2 }, barMaxWidth: 36, emphasis: { focus: "series" },
      label: aggregate ? { show: true, position: "top", distance: 8, color: colors.ink, fontSize: 14, formatter: (params: { value?: unknown }) => compactNumber(params.value) } : undefined,
      labelLayout: aggregate ? { hideOverlap: true } : undefined,
    })),
    dataZoom: zoom ? [{ type: "inside", filterMode: "none" }, { type: "slider", filterMode: "none", bottom: 0, height: 16, showDetail: false }] : [],
  };
}

export function CategoryTokenChart({ data, dimension, granularity, aggregate = false, showMetricControl = true, title: customTitle }: { data: TimePoint[]; dimension: Dimension; granularity: Granularity; aggregate?: boolean; showMetricControl?: boolean; title?: string }) {
  const [metric, setMetric] = useState<TokenMetric>("totalTokens");
  const [mode, setMode] = useState<ChartMode>("bar");
  const subject = dimension === "departments" ? "部门" : "模型";
  const title = customTitle || `${subject} Token 消耗`;
  const available = aggregate ? data.some(point => point[metric] !== null) : data.some(point => point[dimension] !== undefined);
  const count = aggregate ? 0 : categoryTokenSeries(data, dimension, metric).reduce((sum, group) => sum + group.ids.length, 0);
  const metricLabel = metric === "totalTokens" ? "总Token" : "输出Token";
  const description = aggregate ? "汇总所选部门，每个时间段显示总量。" : `按${subject}分类${count > 0 ? `，共 ${count} 个${subject}` : ""}${count > 7 ? "；前 7 项单独展示，其余合并" : ""}；可点击图例筛选。`;
  return <>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 className="m-0 flex items-center gap-2 text-base font-semibold">{title}<HelpTip label={`${title}说明`}>{description}</HelpTip></h2>
      <div className="flex flex-wrap gap-3">
        {showMetricControl && <Segmented value={metric} onChange={setMetric} label={`${subject}消耗指标`} options={[{ value: "totalTokens", label: "总Token" }, { value: "outputTokens", label: "输出Token" }]} />}
        <Segmented value={mode} onChange={setMode} label={`${subject}图表模式`} options={[{ value: "bar", label: aggregate ? "柱状" : "堆叠柱状" }, { value: "line", label: "折线" }]} />
      </div>
    </div>
    {available && (aggregate || count > 0) ? <Chart label={aggregate ? `${title}，所选部门${metricLabel}总量趋势` : `${title}，按${subject}分类的${metricLabel}趋势`} height={280} option={buildCategoryTokenOption(data, dimension, metric, mode, granularity, aggregate)} /> : <Empty title={data.length && !available ? aggregate ? "总量暂不可用" : "分类消耗暂不可用" : "暂无消耗数据"} detail={data.length && !available ? aggregate ? "当前数据未提供逐时间段的 Token 总量。" : "当前数据未提供逐时间段的分类消耗。" : "当前筛选范围没有可展示的记录。"} />}
  </>;
}
