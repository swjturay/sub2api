import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { useRemote } from "./useRemote";
function Harness({loader}:{loader:()=>Promise<string>}){const [filter,setFilter]=useState("近7天"),state=useRemote(()=>loader(),[],false);return <><input aria-label="筛选" value={filter} onChange={e=>setFilter(e.target.value)}/><div data-testid="data">{state.data}</div><div data-testid="error">{state.error}</div></>}
describe("manual refresh",()=>{
 it("keeps stale data and local filters when an offline refresh fails",async()=>{const loader=vi.fn<()=>Promise<string>>().mockResolvedValueOnce("已加载概览").mockRejectedValueOnce(new Error("网络离线"));let successes=0;const success=()=>successes++;window.addEventListener("insights-refresh-success",success);render(<Harness loader={loader}/>);await screen.findByText("已加载概览");fireEvent.change(screen.getByLabelText("筛选"),{target:{value:"自定义区间"}});const successfulAt=successes;window.dispatchEvent(new Event("insights-manual-refresh"));await screen.findByText("网络离线");expect(screen.getByTestId("data")).toHaveTextContent("已加载概览");expect(screen.getByLabelText("筛选")).toHaveValue("自定义区间");expect(successes).toBe(successfulAt);expect(loader).toHaveBeenCalledTimes(2);window.removeEventListener("insights-refresh-success",success)});
});

it("coalesces refreshes while the same request is still running",async()=>{
 let resolve!:(value:string)=>void;
 const loader=vi.fn(()=>new Promise<string>(done=>{resolve=done}));
 const hook=renderHook(()=>useRemote(()=>loader(),[],false));
 await act(async()=>{});
 act(()=>window.dispatchEvent(new Event("insights-manual-refresh")));
 expect(loader).toHaveBeenCalledTimes(1);
 await act(async()=>resolve("完成"));
 expect(hook.result.current.data).toBe("完成");
});

it("aborts the previous request when its dependency changes",async()=>{
 const signals:AbortSignal[]=[];
 const loader=vi.fn((signal:AbortSignal)=>{signals.push(signal);return new Promise<string>(()=>{})});
 const hook=renderHook(({filter}:{filter:string})=>useRemote(signal=>loader(signal),[filter],false),{initialProps:{filter:"a"}});
 await act(async()=>{});
 hook.rerender({filter:"b"});
 await act(async()=>{});
 expect(loader).toHaveBeenCalledTimes(2);
 expect(signals[0].aborted).toBe(true);
});

it("shows scoped cached data immediately while refreshing in the background",async()=>{
 const first=renderHook(()=>useRemote(()=>Promise.resolve("缓存值"),[],false,"user:7:overview"));
 await act(async()=>{});
 expect(first.result.current.data).toBe("缓存值");
 first.unmount();
 const refresh=vi.fn(()=>new Promise<string>(()=>{}));
 const second=renderHook(()=>useRemote(()=>refresh(),[],false,"user:7:overview"));
 expect(second.result.current.data).toBe("缓存值");
 await act(async()=>{});
 expect(refresh).toHaveBeenCalledTimes(1);
 const other=renderHook(()=>useRemote(()=>new Promise<string>(()=>{}),[],false,"user:8:overview"));
 expect(other.result.current.data).toBeNull();
});

it("does not start a deferred request until it is enabled",async()=>{
 const loader=vi.fn(()=>Promise.resolve("延迟数据"));
 const hook=renderHook(({enabled}:{enabled:boolean})=>useRemote(()=>loader(),[],false,"deferred",enabled),{initialProps:{enabled:false}});
 await act(async()=>{});
 expect(loader).not.toHaveBeenCalled();
 hook.rerender({enabled:true});
 await act(async()=>{});
 expect(loader).toHaveBeenCalledTimes(1);
 expect(hook.result.current.data).toBe("延迟数据");
});
