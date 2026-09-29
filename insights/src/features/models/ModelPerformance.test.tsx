import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { adaptModelProfile, insightsApi } from "../../lib/api";
import type { ModelProfile } from "../../lib/types";
import { ModelPerformance } from "./ModelPerformance";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });
const model = adaptModelProfile({ identity: { platform: "antigravity", name: "gemini-3.1-pro" }, performance: { average_ttft_ms: 111 } });
const withLatency = (value: number) => ({ ...model, metrics: { ...model.metrics, ttft: { value } } });

it("aborts superseded windows and does not show old measurements under a new window", async () => {
  let resolveSeven!: (value: ModelProfile) => void;
  const load = vi.spyOn(insightsApi, "model")
    .mockImplementationOnce(() => new Promise(resolve => { resolveSeven = resolve; }))
    .mockResolvedValueOnce(withLatency(240));
  const user = userEvent.setup();
  render(<ModelPerformance model={model} />);
  await user.click(screen.getByRole("button", { name: "近 7 天" }));
  expect(screen.queryByText("111ms")).not.toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("正在加载近 7 天性能");
  const signal = load.mock.calls[0][2];
  await user.click(screen.getByRole("button", { name: "近 24 小时" }));
  expect(await screen.findByText("240ms")).toBeVisible();
  expect(signal?.aborted).toBe(true);
  await act(async () => { resolveSeven(withLatency(777)); });
  expect(screen.getByText("240ms")).toBeVisible();
  expect(screen.queryByText("777ms")).not.toBeInTheDocument();
});

it("offers retry for the selected window and aborts a pending request when details close", async () => {
  const load = vi.spyOn(insightsApi, "model")
    .mockRejectedValueOnce(new Error("性能暂不可用"))
    .mockResolvedValueOnce(withLatency(777));
  const user = userEvent.setup();
  const view = render(<ModelPerformance model={model} />);
  await user.click(screen.getByRole("button", { name: "近 7 天" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("性能暂不可用");
  await user.click(screen.getByRole("button", { name: "重试" }));
  expect(await screen.findByText("777ms")).toBeVisible();
  expect(load.mock.calls[1][1]).toBe("7d");
  load.mockImplementationOnce(() => new Promise(() => undefined));
  await user.click(screen.getByRole("button", { name: "近 24 小时" }));
  const signal = load.mock.calls[2][2];
  view.unmount();
  expect(signal?.aborted).toBe(true);
});
