import { useRef, useState } from "react";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ModelProfile } from "../../lib/types";
import { ModelCard } from "./ModelCard";
import { ModelComparisonDialog } from "./ModelComparisonDialog";
import { ModelDetailDialog } from "./ModelDetailDialog";
import { ModelSelectionTray } from "./ModelSelectionTray";
import { restoreModelDialogFocus } from "./modelDialogFocus";

vi.mock("../../components/charts/Chart", () => ({ Chart: () => null }));
afterEach(cleanup);

const model: ModelProfile = {
  id: "openai:gpt-a",
  name: "gpt-a",
  platform: "openai",
  description: "Model A",
  useCases: [],
  contextLimit: null,
  maxOutput: null,
  inputModalities: [],
  outputModalities: [],
  reasoning: "unknown",
  toolCalling: "unknown",
  structuredOutput: "unknown",
  sources: [],
  updatedAt: null,
  version: 1,
  pricing: [],
  metrics: {
    tpm: { value: null },
    rpm: { value: null },
    ttft: { value: null },
    tpot: { value: null },
  },
};

function ComparisonHarness() {
  const [open, setOpen] = useState(false);
  const opener = useRef<HTMLElement | null>(null);
  return (
    <>
      <input id="model-catalog-search" aria-label="搜索模型" />
      <ModelSelectionTray
        selected={[model, { ...model, id: "openai:gpt-b", name: "gpt-b" }]}
        onRemove={() => undefined}
        onClear={() => undefined}
        onCompare={(element) => { opener.current = element; setOpen(true); }}
      />
      <ModelComparisonDialog
        open={open}
        onOpenChange={setOpen}
        data={null}
        loading={false}
        error={null}
        retry={() => undefined}
        remove={() => undefined}
        window="24h"
        restoreFocusElement={opener.current}
      />
    </>
  );
}

function DetailHarness() {
  const [detail, setDetail] = useState(false);
  const opener = useRef<HTMLElement | null>(null);
  return (
    <>
      <input id="model-catalog-search" aria-label="搜索模型" />
      <ModelCard
        model={model}
        selected={false}
        selectionDisabled={false}
        onToggle={() => undefined}
        onOpen={(element) => { opener.current = element; setDetail(true); }}
      />
      {detail && (
        <ModelDetailDialog
          model={model}
          close={() => setDetail(false)}
          restoreFocusElement={opener.current}
        />
      )}
    </>
  );
}

function DisabledComparisonOpenerHarness() {
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState([model, { ...model, id: "openai:gpt-b", name: "gpt-b" }]);
  const opener = useRef<HTMLElement | null>(null);
  return (
    <>
      <input id="model-catalog-search" aria-label="搜索模型" />
      <ModelSelectionTray
        selected={selected}
        onRemove={() => undefined}
        onClear={() => undefined}
        onCompare={(element) => {
          opener.current = element;
          setSelected((current) => current.slice(0, 1));
          setOpen(true);
        }}
      />
      <ModelComparisonDialog
        open={open}
        onOpenChange={setOpen}
        data={null}
        loading={false}
        error={null}
        retry={() => undefined}
        remove={() => undefined}
        window="24h"
        restoreFocusElement={opener.current}
      />
    </>
  );
}

describe("model dialog focus restoration", () => {
  it("keeps full pricing conditions and safe sources available in the detail drawer", async () => {
    const user = userEvent.setup();
    render(<ModelDetailDialog model={{
      ...model,
      pricing: [{ label: "input", value: "0.000005 USD/token", condition: "max_tokens=272000,tier=≤272K" }],
      sources: [
        { label: "官方规格", url: "https://example.com/spec", updatedAt: null },
        { label: "无效来源", url: "javascript:alert(1)", updatedAt: null },
      ],
    }} close={() => undefined} restoreFocusElement={null} />);
    expect(screen.getByText("实测性能")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "价格与来源" }));
    expect(screen.getByText("5 USD/百万 Token")).toBeVisible();
    expect(screen.getByText("272,000")).toBeVisible();
    expect(screen.getByText("≤272K")).toBeVisible();
    expect(screen.getByRole("link", { name: "官方规格" })).toHaveAttribute("href", "https://example.com/spec");
    expect(screen.queryByRole("link", { name: "无效来源" })).not.toBeInTheDocument();
  });

  it("returns comparison focus to the tray action after Escape", async () => {
    const user = userEvent.setup();
    render(<ComparisonHarness />);
    const opener = screen.getByRole("button", { name: /对比 2 个模型/ });
    await user.click(opener);
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(opener).toHaveFocus());
  });

  it("returns detail focus to the card action after Escape", async () => {
    const user = userEvent.setup();
    render(<DetailHarness />);
    const opener = screen.getByRole("button", { name: /查看详情/ });
    await user.click(opener);
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(opener).toHaveFocus());
  });

  it("falls back to catalog search when an opener is no longer connected", () => {
    render(<input id="model-catalog-search" aria-label="搜索模型" />);
    const event = { preventDefault: vi.fn() };
    restoreModelDialogFocus(event, document.createElement("button"));
    expect(event.preventDefault).toHaveBeenCalledOnce();
    expect(screen.getByRole("textbox", { name: "搜索模型" })).toHaveFocus();
  });

  it("falls back to catalog search when the tray action becomes disabled", async () => {
    const user = userEvent.setup();
    render(<DisabledComparisonOpenerHarness />);
    const opener = screen.getByRole("button", { name: /对比 2 个模型/ });
    await user.click(opener);
    expect(opener).toBeDisabled();
    await user.keyboard("{Escape}");
    const search = screen.getByRole("textbox", { name: "搜索模型" });
    await waitFor(() => expect(search).toHaveFocus());
    expect(document.activeElement).not.toBe(document.body);
  });
});
