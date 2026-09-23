import type { EChartsOption } from "echarts";
import type { HeatmapDay, GatewayAnalytics, ModelComparison } from "../lib/types";
import { cartesianTheme, chartColors, compactNumber, compactTimeLabel, fullNumber, heatPalette, legendTheme, palette, tooltipTheme } from "../components/charts/chartTheme";
import { percent } from "../lib/format";

function escapeHtml(value: unknown) {
  return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;");
}

export function heatmapSelectionDate(p: unknown, days: HeatmapDay[]): string | null {
  const date=(p as {data?:[string,number]}).data?.[0];
  const day=date ? days.find((d)=>d.date===date) : undefined;
  return day && (day.state==="value" || day.state==="zero") ? day.date : null;
}
export function buildHeatmapOption(days: HeatmapDay[], thresholds:number[], year:number): EChartsOption {
  const colors = chartColors();
  const levels = [...new Set(thresholds.filter((v)=>Number.isFinite(v) && v>0))].sort((a,b)=>a-b).slice(0,4);
  const pieces = levels.map((max,i)=>{
    const previous=i===0?0:levels[i-1];
    const first=previous+1;
    const colorIndex=Math.max(0,Math.ceil((i+1)*heatPalette.length/levels.length)-1);
    return {gt:previous,lte:max,color:heatPalette[colorIndex],label:first===max?compactNumber(max):`${compactNumber(first)}–${compactNumber(max)}`};
  });
  const named = (state:HeatmapDay["state"]) => days.filter(d=>d.state===state).map(d=>[d.date,d.tokens ?? 0,state]);
  return {
    tooltip:{ ...tooltipTheme(), formatter:(p:unknown)=>{ const d=(p as {data:[string,number,string]}).data; const labels:Record<string,string>={zero:"已观测：0 Token",missing:"数据缺口",out_of_scope:"统计范围外",future:"未来日期"}; return `<b>${escapeHtml(d[0])}</b><br/>${d[2]==="value"?`${d[1].toLocaleString("zh-CN")} Token`:labels[d[2]]}`; }},
    visualMap:{ type:"piecewise", seriesIndex:0, dimension:1, pieces, orient:"horizontal", left:"center", bottom:0, itemWidth:14, itemHeight:10, itemGap:10, textStyle:{color:colors.muted,fontSize:12} },
    calendar:{ range:String(year), cellSize:["auto",17], top:32, left:42, right:16, bottom:42, yearLabel:{show:false}, dayLabel:{firstDay:1,color:colors.muted,fontSize:10}, monthLabel:{nameMap:"ZH",color:colors.muted,fontSize:10}, splitLine:{show:false}, itemStyle:{color:colors.surfaceSubtle,borderColor:colors.surface,borderWidth:3}},
    series:[
      {name:"非零",type:"heatmap",coordinateSystem:"calendar",data:named("value")},
      {name:"零",type:"heatmap",coordinateSystem:"calendar",data:named("zero"),itemStyle:{color:colors.surfaceSubtle,borderColor:colors.surface,borderWidth:3}},
      {name:"数据缺口",type:"heatmap",coordinateSystem:"calendar",data:named("missing"),itemStyle:{color:colors.border,borderColor:colors.surface,borderWidth:3,decal:{symbol:"rect",dashArrayX:[1,2],dashArrayY:[2,3]}}},
      {name:"统计范围外",type:"heatmap",coordinateSystem:"calendar",data:named("out_of_scope"),itemStyle:{color:"rgba(148,163,184,.12)",borderColor:colors.surface,borderWidth:3}},
      {name:"未来",type:"heatmap",coordinateSystem:"calendar",data:named("future"),itemStyle:{color:"rgba(148,163,184,.08)",borderColor:colors.border,borderWidth:1}},
    ]
  };
}
export function buildFunnelOption(funnel:GatewayAnalytics["funnel"]):EChartsOption {
 const colors=chartColors(),maximum=Math.max(1,...funnel.flatMap(x=>x.count===null?[]:[x.count]));
 type Datum={name:string;value:number|null;share:number|null;status?:string};
 const statusText=(status?:string)=>status==="pending"?"待观察":status==="partial"?"不完整":status==="unknown"?"数据不足":"";
 return {
  color:["#4f46e5","#6366f1","#818cf8","#a5b4fc","#c7d2fe"],
  tooltip:{...tooltipTheme(),trigger:"item",formatter:(p:unknown)=>{
   const d=(p as {data:Datum}).data,status=statusText(d.status);
   return `<b>${escapeHtml(d.name)}</b><br/>${d.value===null?"—":`${fullNumber(d.value)} 人`}${d.share===null?"":`<br/>占总用户 ${percent(d.share)}`}${status?`<br/>${status}`:""}`;
  }},
  series:[{
   type:"funnel",sort:"none",min:0,max:maximum,minSize:"0%",maxSize:"100%",left:"22%",top:18,bottom:12,width:"72%",gap:3,
   label:{position:"left",color:colors.ink,fontSize:12,formatter:(p:unknown)=>{
    const d=(p as {data:Datum}).data,status=statusText(d.status);
    return `${d.name}\n${d.value===null?"—":`${compactNumber(d.value)} 人`}${status?`\n${status}`:""}`;
   }},
   labelLine:{length:12,lineStyle:{color:colors.border}},itemStyle:{borderColor:colors.surface,borderWidth:2,borderRadius:4},emphasis:{focus:"self"},
   data:funnel.map(x=>({name:x.label,value:x.count,share:x.share,status:x.status,itemStyle:x.count===null?{opacity:.32,color:colors.muted}:undefined}))
  }] as EChartsOption["series"]
 };
}
export function buildPreferenceOption(preferences:GatewayAnalytics["preferences"]):EChartsOption {
 const models=[...new Set(preferences.flatMap(x=>x.models.map(m=>m.model)))];
 const colors=chartColors(),theme=cartesianTheme(58);
 return {color:palette,textStyle:theme.textStyle,tooltip:{...tooltipTheme(),trigger:"item",formatter:(p:unknown)=>{const d=p as {seriesName:string,data:{value:number,count:number}};return `<b>${escapeHtml(d.seriesName)}</b><br/>${fullNumber(d.data.count)} 次 · ${percent(d.data.value/100)}`;}},legend:{...legendTheme(),bottom:0},grid:{...(theme.grid as object),left:16,right:24,top:16,bottom:66},xAxis:{...theme.xAxis,type:"value",max:100,axisLabel:{...(theme.xAxis.axisLabel as object),formatter:"{value}%"}},yAxis:{...theme.yAxis,type:"category",data:preferences.map(x=>`${x.department}  ·  ${x.total.toLocaleString("zh-CN")}`),axisLabel:{...(theme.yAxis.axisLabel as object),width:116,overflow:"truncate",color:colors.ink}},series:models.map((model,index)=>({name:model,type:"bar",stack:"total",barMaxWidth:24,emphasis:{focus:"series"},itemStyle:{borderRadius:index===0?[4,0,0,4]:index===models.length-1?[0,4,4,0]:0},data:preferences.map(dep=>{const v=dep.models.find(m=>m.model===model);return v?.share===null||v?.share===undefined?null:{value:v.share*100,count:v.count};})}))};
}
export function buildComparisonTrendOption(data:ModelComparison, metric:"tpm"|"rpm"|"ttft"|"tpot"):EChartsOption {
 const theme=cartesianTheme(58);
 return {color:palette,textStyle:theme.textStyle,tooltip:{...tooltipTheme(),trigger:"axis",axisPointer:{type:"line",lineStyle:{color:palette[0],opacity:.28}},valueFormatter:v=>v===null||v===undefined?"—":fullNumber(v)},legend:{...legendTheme(),bottom:0},grid:{...(theme.grid as object),bottom:64},xAxis:{...theme.xAxis,type:"category",boundaryGap:false,data:data.trends.map(x=>x.at),axisLabel:{...(theme.xAxis.axisLabel as object),formatter:compactTimeLabel,hideOverlap:true}},yAxis:{...theme.yAxis,type:"value",axisLabel:{...(theme.yAxis.axisLabel as object),formatter:compactNumber}},series:data.models.map(m=>({name:m.name,type:"line",connectNulls:false,showSymbol:data.trends.length<=8,symbolSize:6,smooth:.18,lineStyle:{width:2.25},emphasis:{focus:"series",lineStyle:{width:3}},data:data.trends.map(t=>t.values.find(v=>v.model===m.id)?.performance[metric] ?? null)}))};
}
