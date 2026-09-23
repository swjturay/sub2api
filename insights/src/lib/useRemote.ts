/* eslint-disable react-hooks/exhaustive-deps */
import { useCallback, useEffect, useRef, useState } from "react";
import { REFRESH_INTERVAL_MS } from "./config";
export function useRemote<T>(loader:(signal:AbortSignal)=>Promise<T>,deps:unknown[],auto=true){
 const [data,setData]=useState<T|null>(null),[error,setError]=useState<string|null>(null),[loading,setLoading]=useState(true),[updatedAt,setUpdatedAt]=useState<Date|null>(null); const generation=useRef(0),dataRef=useRef<T|null>(null);dataRef.current=data;
 const refresh=useCallback(async()=>{const current=++generation.current,controller=new AbortController();setLoading(dataRef.current===null);try{const next=await loader(controller.signal);if(current===generation.current){setData(next);setError(null);const at=new Date();setUpdatedAt(at);window.dispatchEvent(new CustomEvent("insights-refresh-success",{detail:{at}}))}}catch(e){if(current===generation.current&&(e as Error).name!=="AbortError")setError((e as Error).message)}finally{if(current===generation.current){setLoading(false);window.dispatchEvent(new CustomEvent("insights-refresh-settled"))}}return()=>controller.abort()},deps);
 useEffect(()=>{void refresh()},[refresh]);
 useEffect(()=>{const manual=()=>void refresh();window.addEventListener("insights-manual-refresh",manual);return()=>window.removeEventListener("insights-manual-refresh",manual)},[refresh]);
 useEffect(()=>{if(!auto)return;let timer:number|undefined;const schedule=()=>{window.clearInterval(timer);if(!document.hidden)timer=window.setInterval(()=>void refresh(),REFRESH_INTERVAL_MS)},visible=()=>{if(!document.hidden)void refresh();schedule()};schedule();document.addEventListener("visibilitychange",visible);return()=>{window.clearInterval(timer);document.removeEventListener("visibilitychange",visible)}},[auto,refresh]);
 return {data,error,loading,updatedAt,refresh};
}
