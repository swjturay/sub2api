import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { useRemote } from "./useRemote";
function Harness({loader}:{loader:()=>Promise<string>}){const [filter,setFilter]=useState("近7天"),state=useRemote(()=>loader(),[],false);return <><input aria-label="筛选" value={filter} onChange={e=>setFilter(e.target.value)}/><div data-testid="data">{state.data}</div><div data-testid="error">{state.error}</div></>}
describe("manual refresh",()=>{
 it("keeps stale data and local filters when an offline refresh fails",async()=>{const loader=vi.fn<()=>Promise<string>>().mockResolvedValueOnce("已加载概览").mockRejectedValueOnce(new Error("网络离线"));let successes=0;const success=()=>successes++;window.addEventListener("insights-refresh-success",success);render(<Harness loader={loader}/>);await screen.findByText("已加载概览");fireEvent.change(screen.getByLabelText("筛选"),{target:{value:"自定义区间"}});const successfulAt=successes;window.dispatchEvent(new Event("insights-manual-refresh"));await screen.findByText("网络离线");expect(screen.getByTestId("data")).toHaveTextContent("已加载概览");expect(screen.getByLabelText("筛选")).toHaveValue("自定义区间");expect(successes).toBe(successfulAt);expect(loader).toHaveBeenCalledTimes(2);window.removeEventListener("insights-refresh-success",success)});
});
