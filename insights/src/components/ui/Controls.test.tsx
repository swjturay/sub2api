import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Segmented, Toggle } from "./Controls";
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
});
