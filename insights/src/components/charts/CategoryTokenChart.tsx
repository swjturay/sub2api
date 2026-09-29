/* eslint-disable react-refresh/only-export-components */
import { useState } from "react";
import type { EChartsOption } from "echarts";
import type { ChartMode, TimePoint } from "../../lib/types";
import { Chart, palette } from "./Chart";
import { cartesianTheme, compactNumber, compactTimeLabel, fullNumber, legendTheme, tooltipTheme } from "./chartTheme";
import { Segmented } from "../ui/Controls";
import { Empty } from "../ui/States";

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

export function buildCategoryTokenOption(data: TimePoint[], dimension: Dimension, metric: TokenMetric, mode: ChartMode): EChartsOption {
  const groups = categoryTokenSeries(data, dimension, metric);
  const zoom = data.length > 16;
  const theme = cartesianTheme(zoom ? 90 : 62);
  return {
    color: palette, textStyle: theme.textStyle, grid: theme.grid,
    legend: { ...legendTheme(), bottom: zoom ? 26 : 0 },
    tooltip: { ...tooltipTheme(), trigger: "axis", axisPointer: { type: mode === "bar" ? "shadow" : "line" }, valueFormatter: value => value == null ? "—" : fullNumber(value) },
    xAxis: { ...theme.xAxis, type: "category", boundaryGap: mode === "bar", data: data.map(point => point.bucket), axisLabel: { ...(theme.xAxis.axisLabel as object), hideOverlap: true, formatter: (value: string, index: number) => `${compactTimeLabel(value)}${data[index]?.incomplete ? "\n未结束" : ""}` } },
    yAxis: { ...theme.yAxis, type: "value", min: 0, axisLabel: { ...(theme.yAxis.axisLabel as object), formatter: compactNumber } },
    series: groups.map(group => ({
      id: group.id, name: group.name, type: mode, stack: mode === "bar" ? "tokens" : undefined,
      data: group.values, connectNulls: false, showSymbol: data.length <= 8, symbolSize: 5,
      lineStyle: { width: 2 }, barMaxWidth: 36, emphasis: { focus: "series" },
    })),
    dataZoom: zoom ? [{ type: "inside", filterMode: "none" }, { type: "slider", filterMode: "none", bottom: 0, height: 16, showDetail: false }] : [],
  };
}

export function CategoryTokenChart({ data, dimension }: { data: TimePoint[]; dimension: Dimension }) {
  const [metric, setMetric] = useState<TokenMetric>("totalTokens");
  const [mode, setMode] = useState<ChartMode>("bar");
  const subject = dimension === "departments" ? "部门" : "模型";
  const title = `${subject} Token 消耗`;
  const available = data.some(point => point[dimension] !== undefined);
  const count = categoryTokenSeries(data, dimension, metric).reduce((sum, group) => sum + group.ids.length, 0);
  return <>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 className="m-0 text-base font-semibold">{title}</h2>
        <p className="mb-0 mt-1 text-xs muted">按{subject}分类{count > 0 && ` · ${count} 个${subject}`}{count > 7 && " · 前 7 项单独展示，其余合并"} · 点击图例筛选</p>
      </div>
      <div className="flex flex-wrap gap-3">
        <Segmented value={metric} onChange={setMetric} label={`${subject}消耗指标`} options={[{ value: "totalTokens", label: "总Token" }, { value: "outputTokens", label: "输出Token" }]} />
        <Segmented value={mode} onChange={setMode} label={`${subject}图表模式`} options={[{ value: "bar", label: "堆叠柱状" }, { value: "line", label: "折线" }]} />
      </div>
    </div>
    {available && count > 0 ? <Chart label={`${title}，按${subject}分类的${metric === "totalTokens" ? "总Token" : "输出Token"}趋势`} height={280} option={buildCategoryTokenOption(data, dimension, metric, mode)} /> : <Empty title={data.length && !available ? "分类消耗暂不可用" : "暂无消耗数据"} detail={data.length && !available ? "当前数据未提供逐时间段的分类消耗。" : "当前筛选范围没有可展示的记录。"} />}
  </>;
}
