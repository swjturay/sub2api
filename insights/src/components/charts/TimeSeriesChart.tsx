/* eslint-disable react-refresh/only-export-components */
import type { EChartsOption } from "echarts";
import type { ChartMode, TimePoint } from "../../lib/types";
import { Chart, palette } from "./Chart";
import { cartesianTheme, compactNumber, compactTimeLabel, fullNumber, tooltipTheme } from "./chartTheme";
import { percent as formatPercent } from "../../lib/format";

export function buildTimeSeriesOption(data:TimePoint[],metric:"totalTokens"|"outputTokens"|"cacheHitRate"|"successRate"|"users"|"requests",mode:ChartMode,percent=false):EChartsOption {
  const values=data.map(point=>{const value=point[metric];return typeof value==="number"?(percent?value*100:value):null});
  const theme=cartesianTheme(data.length>16?74:42);
  return {
    color:[palette[0]],textStyle:theme.textStyle,
    tooltip:{...tooltipTheme(),trigger:"axis",axisPointer:{type:"line",lineStyle:{color:palette[0],opacity:.35}},valueFormatter:value=>value===null||value===undefined?"—":percent?formatPercent(Number(value)/100):fullNumber(value)},
    grid:theme.grid,
    xAxis:{...theme.xAxis,type:"category",boundaryGap:mode==="bar",data:data.map(point=>point.bucket),axisLabel:{...(theme.xAxis.axisLabel as object),formatter:compactTimeLabel,hideOverlap:true}},
    yAxis:{...theme.yAxis,type:"value",axisLabel:{...(theme.yAxis.axisLabel as object),formatter:percent?"{value}%":compactNumber}},
    series:[{type:mode,data:values.map((value,index)=>data[index].incomplete?{value,symbol:"emptyCircle",symbolSize:8,itemStyle:{borderWidth:2}}:value),smooth:mode==="line"?.22:false,showSymbol:data.length<=8||data.some(point=>point.incomplete),symbolSize:6,lineStyle:{width:2.25},itemStyle:{borderRadius:mode==="bar"?[4,4,1,1]:0},barMaxWidth:28,areaStyle:mode==="line"?{opacity:.08}:undefined,emphasis:{focus:"series",lineStyle:{width:3}},markPoint:{symbol:"circle",symbolSize:10,label:{show:true,formatter:"未结束",position:"top",distance:6,fontSize:10},data:data.flatMap((point,index)=>point.incomplete&&values[index]!==null?[{name:"未结束",coord:[point.bucket,values[index]]}]:[])}}] as EChartsOption["series"],
    dataZoom:data.length>16?[{type:"inside",filterMode:"none"},{type:"slider",filterMode:"none",height:16,bottom:8,borderColor:"transparent",showDetail:false,brushSelect:false}]:[]
  };
}
export function TimeSeriesChart({data,metric,mode,label,percent=false}:{data:TimePoint[];metric:"totalTokens"|"outputTokens"|"cacheHitRate"|"successRate"|"users"|"requests";mode:ChartMode;label:string;percent?:boolean}){return <Chart label={label} option={buildTimeSeriesOption(data,metric,mode,percent)}/>}
