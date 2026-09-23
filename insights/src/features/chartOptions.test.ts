/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, expect, it } from "vitest";
import { buildComparisonTrendOption, buildFunnelOption, buildHeatmapOption, heatmapSelectionDate, buildPreferenceOption } from "./chartOptions";
import type { ModelComparison } from "../lib/types";
describe("chart contract options",()=>{
 it("keeps heatmap zero, missing and future separate with at most four nonzero levels",()=>{const o=buildHeatmapOption([{date:"2026-01-01",tokens:1,state:"value"},{date:"2026-01-02",tokens:0,state:"zero"},{date:"2026-01-03",tokens:null,state:"missing"},{date:"2026-12-31",tokens:null,state:"future"}],[10,100,1000,10000],2026) as any;expect(o.series).toHaveLength(4);expect(o.visualMap.pieces).toHaveLength(4);expect(o.visualMap.dimension).toBe(1);expect(o.series.map((s:any)=>s.name)).toEqual(["非零","零","缺失","未来"])});
 it("preserves funnel layer order and truthful widths",()=>{const o=buildFunnelOption([{key:"a",label:"总",count:100,share:1},{key:"b",label:"次日",count:1,share:.01},{key:"c",label:"未知",count:null,share:null}]) as any;expect(o.series[0].sort).toBe("none");expect(o.series[0].minSize).toBe("0%");expect(o.series[0].data.map((x:any)=>x.name)).toEqual(["总","次日","未知"]);expect(o.series[0].data[2].value).toBeNull()});
 it("keeps exact tooltip counts and tiny positive funnel shares",()=>{const o=buildFunnelOption([{key:"a",label:"总",count:100000,share:.000001}]) as any;const html=o.tooltip.formatter({data:o.series[0].data[0]});expect(html).toContain("100,000 人");expect(html).toContain("0.0001%");expect(html).not.toContain("0.0%")});
 it("keeps missing preference ratios as gaps and small values nonzero",()=>{const o=buildPreferenceOption([{department:"研发",total:3,models:[{model:"a",count:0,share:null},{model:"b",count:1,share:.0001}]}]) as any;expect(o.series[0].data).toEqual([null]);expect(o.series[1].data[0].value).toBe(.01)});
 it("escapes user-defined model labels in HTML tooltips",()=>{const o=buildPreferenceOption([{department:"研发",total:1,models:[{model:'<img onerror=window.__insightsXSS=1>',count:1,share:1}]}]) as any;const html=o.tooltip.formatter({seriesName:'<img onerror=window.__insightsXSS=1>',data:{value:100,count:1}});expect(html).toContain("&lt;img");expect(html).not.toContain("<img")});
 it("keeps exact preference counts and tiny positive shares",()=>{const o=buildPreferenceOption([{department:"研发",total:100000,models:[{model:"a",count:100000,share:.000001}]}]) as any;const html=o.tooltip.formatter({seriesName:"a",data:o.series[0].data[0]});expect(html).toContain("100,000 次");expect(html).toContain("0.0001%");expect(html).not.toContain("0.0%")});
 it("constructs all four comparison trend series",()=>{const base:any={id:"p:a",name:"A",platform:"p"};const d:ModelComparison={models:[base],trends:[{at:"t",complete:true,values:[{model:"p:a",performance:{tpm:1,rpm:2,ttft:3,tpot:4}}]}]};expect((buildComparisonTrendOption(d,"tpot") as any).series[0].data).toEqual([4])});
});


it("selects only observed heatmap dates",()=>{
  const days:any=[{date:"2026-09-22",tokens:533,state:"value"},{date:"2026-09-23",tokens:null,state:"future"},{date:"2026-09-21",tokens:0,state:"zero"}];
  expect(heatmapSelectionDate({data:["2026-09-22",533]},days)).toBe("2026-09-22");
  expect(heatmapSelectionDate({data:["2026-09-23",0]},days)).toBeNull();
  expect(heatmapSelectionDate({data:["2026-09-21",0]},days)).toBe("2026-09-21");
});

it("does not put zero in nonzero heat bins or overlap shared boundaries",()=>{
 const o=buildHeatmapOption([], [10,100,1000,10000],2026) as any;
 expect(o.visualMap.pieces[0]).toMatchObject({gt:0,lte:10});
 expect(o.visualMap.pieces[1]).toMatchObject({gt:10,lte:100});
 const tiny=buildHeatmapOption([], [1,1,1,1],2026) as any;
 expect(tiny.visualMap.pieces).toHaveLength(1);
 expect(tiny.visualMap.pieces[0]).toMatchObject({gt:0,lte:1});
});

it("exposes pending and unknown retention without claiming zero loss",()=>{
 const o=buildFunnelOption([{key:"a",label:"次日留存",count:0,share:0,status:"pending"},{key:"b",label:"30日留存",count:null,share:null,status:"unknown"}]) as any;
 expect(o.series[0].label.formatter({data:o.series[0].data[0]})).toContain("待观察");
 const unknown=o.tooltip.formatter({data:o.series[0].data[1]});
 expect(unknown).toContain("数据不足");expect(unknown).not.toContain("0 人");expect(unknown).not.toContain("0.0%");
});
