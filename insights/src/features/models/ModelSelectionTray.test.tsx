import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ModelSelectionTray } from "./ModelSelectionTray";

const models = [
  { id: "openai:gpt-a", name: "gpt-a", platform: "openai" },
  { id: "anthropic:claude-b", name: "claude-b", platform: "anthropic" },
];

describe("model selection tray", () => {
  it("requires two selections before comparison and exposes removal", async () => {
    const compare = vi.fn();
    const remove = vi.fn();
    const { rerender } = render(
      <ModelSelectionTray selected={models.slice(0, 1)} onRemove={remove} onClear={vi.fn()} onCompare={compare} />,
    );
    expect(screen.getByRole("button", { name: /对比 1 个模型/ })).toBeDisabled();
    rerender(<ModelSelectionTray selected={models} onRemove={remove} onClear={vi.fn()} onCompare={compare} />);
    await userEvent.click(screen.getByRole("button", { name: "移除 gpt-a" }));
    expect(remove).toHaveBeenCalledWith("openai:gpt-a");
    await userEvent.click(screen.getByRole("button", { name: /对比 2 个模型/ }));
    expect(compare).toHaveBeenCalledOnce();
  });
});
