import { cleanup, render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { insightsApi, adaptModelProfile } from "../lib/api";
import { ModelsPage } from "./ModelsPage";

vi.mock("../components/charts/Chart", () => ({ Chart: () => null }));
const originalScrollIntoView = Object.getOwnPropertyDescriptor(Element.prototype, "scrollIntoView");
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  if (originalScrollIntoView) Object.defineProperty(Element.prototype, "scrollIntoView", originalScrollIntoView);
  else Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

it("retains comparison selection through pagination, filtering and switching to the table", async () => {
  const models = Array.from({ length: 14 }, (_, index) => adaptModelProfile({ identity: { platform: index % 2 ? "anthropic" : "openai", name: `Model ${String(index).padStart(2, "0")}` }, profile: {}, performance: {} }));
  vi.spyOn(insightsApi, "models").mockResolvedValue(models);
  const scroll = vi.fn();
  Object.defineProperty(Element.prototype, "scrollIntoView", { configurable: true, value: scroll });
  const user = userEvent.setup();
  render(<ModelsPage auto={false} />);
  await screen.findByRole("button", { name: "选择 Model 00 对比" });
  expect(screen.getAllByRole("article")).toHaveLength(12);
  await user.click(screen.getByRole("button", { name: "选择 Model 00 对比" }));
  await user.click(screen.getByRole("button", { name: "下一页模型" }));
  expect(screen.getAllByRole("article")).toHaveLength(2);
  expect(scroll).toHaveBeenCalledOnce();
  await user.click(within(screen.getByRole("complementary", { name: "模型分类筛选" })).getByRole("button", { name: /Anthropic/ }));
  expect(screen.getAllByRole("article")).toHaveLength(7);
  expect(screen.getByRole("button", { name: "上一页模型" })).toBeDisabled();
  expect(within(screen.getByRole("complementary", { name: "模型对比选择" })).getByText("Model 00")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "列表" }));
  expect(screen.getByRole("table")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "选择 Model 01 对比" }));
  expect(screen.getByRole("button", { name: "对比 2 个模型" })).toBeEnabled();
});

it("keeps the four-model limit and offers recovery from an empty search", async () => {
  vi.spyOn(insightsApi, "models").mockResolvedValue(Array.from({ length: 5 }, (_, index) => adaptModelProfile({ identity: { platform: "openai", name: `M${index}` }, profile: {}, performance: {} })));
  const user = userEvent.setup();
  render(<ModelsPage auto={false} />);
  await screen.findByRole("button", { name: "选择 M0 对比" });
  for (let index = 0; index < 4; index++) await user.click(screen.getByRole("button", { name: `选择 M${index} 对比` }));
  expect(screen.getByRole("button", { name: "选择 M4 对比" })).toBeDisabled();
  await user.type(screen.getByRole("textbox", { name: "搜索模型" }), "missing");
  expect(screen.getByText("没有匹配模型")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "清除筛选" }));
  expect(screen.getAllByRole("article")).toHaveLength(5);
  expect(screen.getByRole("button", { name: "对比 4 个模型" })).toBeEnabled();
});

it("uses vendor identity and keeps the independent performance window inside model details", async () => {
  const gemini = adaptModelProfile({ identity: { platform: "antigravity", name: "gemini-3.1-pro" }, performance: { average_ttft_ms: 111 } });
  const claude = adaptModelProfile({ identity: { platform: "antigravity", name: "claude-sonnet-4-6" } });
  const catalog = vi.spyOn(insightsApi, "models").mockResolvedValue([gemini, claude]);
  const detail = vi.spyOn(insightsApi, "model").mockImplementation(async (_id, window) => ({ ...gemini, metrics: { ...gemini.metrics, ttft: { value: window === "7d" ? 777 : 111 } } }));
  const user = userEvent.setup();
  render(<ModelsPage auto={false} />);
  await screen.findByRole("article", { name: "Gemini · gemini-3.1-pro" });
  expect(screen.queryByText(/Antigravity|有参考价格|全平台聚合性能|平台数量来自配置目录/)).not.toBeInTheDocument();
  expect(screen.queryByRole("group", { name: "性能窗口" })).not.toBeInTheDocument();
  await user.click(within(screen.getByRole("complementary", { name: "模型分类筛选" })).getByRole("button", { name: "Gemini 1" }));
  expect(screen.getAllByRole("article")).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "查看详情" }));
  const dialog = await screen.findByRole("dialog");
  expect(detail).toHaveBeenCalledWith("antigravity:gemini-3.1-pro", "24h");
  expect(within(dialog).getByRole("button", { name: "近 24 小时" })).toHaveAttribute("aria-pressed", "true");
  await user.click(within(dialog).getByRole("button", { name: "近 7 天" }));
  expect(await within(dialog).findByText("777ms")).toBeVisible();
  await user.click(within(dialog).getByRole("button", { name: "价格与来源" }));
  await user.click(within(dialog).getByRole("button", { name: "规格与性能" }));
  expect(within(dialog).getByRole("button", { name: "近 7 天" })).toHaveAttribute("aria-pressed", "true");
  expect(catalog.mock.calls.every(([window]) => window === "24h")).toBe(true);
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByText("111ms")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "查看详情" }));
  expect(within(await screen.findByRole("dialog")).getByRole("button", { name: "近 24 小时" })).toHaveAttribute("aria-pressed", "true");
});
