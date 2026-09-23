import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { MultiSelect } from "./MultiSelect";
import { Segmented, Select, Toggle } from "./Controls";

globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as typeof ResizeObserver;
Object.defineProperties(HTMLElement.prototype, {
  hasPointerCapture: { configurable: true, value: () => false },
  setPointerCapture: { configurable: true, value: () => undefined },
  releasePointerCapture: { configurable: true, value: () => undefined },
  scrollIntoView: { configurable: true, value: () => undefined },
});
afterEach(cleanup);
describe("analysis controls", () => {
  it("changes chart mode without navigation", async () => {
    const change = vi.fn();
    render(
      <Segmented
        value="line"
        onChange={change}
        label="模式"
        options={[
          { value: "line", label: "折线" },
          { value: "bar", label: "柱状" },
        ]}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "柱状" }));
    expect(change).toHaveBeenCalledWith("bar");
  });
  it("exposes automatic refresh as a switch", async () => {
    const change = vi.fn();
    render(<Toggle checked onChange={change} label="自动刷新" />);
    await userEvent.click(screen.getByRole("switch"));
    expect(change).toHaveBeenCalledWith(false);
  });
  it("keeps native option children, empty values, and text fallback compatible", async () => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<Select aria-label="Page size" value={20} onChange={change}><option>20</option><option><>50</></option><option>100</option></Select>);
    await user.click(screen.getByRole("combobox", { name: "Page size" }));
    await user.click(screen.getByRole("option", { name: "50" }));
    expect(change.mock.calls[0][0].target.value).toBe("50");
  });
  it("searches and exposes actual multi-select state separately from keyboard highlight", async () => {
    const change = vi.fn();
    const user = userEvent.setup();
    function Harness() {
      const [value, setValue] = useState<string[]>([]);
      return <MultiSelect label="Models" value={value} onChange={(next) => { setValue(next); change(next); }} options={[{ value: "gpt", label: "GPT", keywords: "OpenAI" }, { value: "claude", label: "Claude", keywords: "Anthropic" }]} />;
    }
    render(<Harness />);
    await user.click(screen.getByRole("combobox", { name: "Models" }));
    const search = screen.getByRole("combobox", { name: "Models搜索" });
    expect(screen.getByRole("checkbox", { name: "GPT 未选择" })).not.toBeChecked();
    await user.type(search, "Anthropic");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(change).toHaveBeenCalledWith(["claude"]);
    expect(screen.getByRole("checkbox", { name: "Claude 已选择" })).toBeChecked();
    expect(search).toBeVisible();
  });
});
