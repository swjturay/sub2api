import { useState } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "../components/ui/Dialog";
import { Select } from "../components/ui/Controls";
import type { CostContributor } from "../lib/types";
import { CostContributorSelect } from "./CostContributorSelect";

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

const contributors: CostContributor[] = [
  { id: 1, name: "王明", email: "wang.eng@example.com", status: "active", departmentId: "eng", department: "研发部" },
  { id: 2, name: "王明", email: "wang.ops@example.com", status: "active", departmentId: "ops", department: "运营部" },
  { id: 3, name: "李华", email: "li@example.com", status: "active", departmentId: "eng", department: "研发部" },
];

function Harness({ change, initial = "1" }: { change: (value: string) => void; initial?: string }) {
  const [value, setValue] = useState(initial);
  return <Dialog defaultOpen><DialogContent><DialogTitle>账号月记录</DialogTitle><DialogDescription>编辑月度信息</DialogDescription><CostContributorSelect value={value} onChange={(next) => { setValue(next); change(next); }} contributors={contributors} /><Select aria-label="付费方式" value="subscription"><option value="subscription">订阅</option><option value="payg">即用即付</option></Select></DialogContent></Dialog>;
}

describe("CostContributorSelect", () => {
  it.each([["李华", "li@example.com"], [" WANG.OPS@EXAMPLE.COM ", "wang.ops@example.com"], ["运营部", "wang.ops@example.com"]])("searches by %s inside the editor dialog", async (query, email) => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<Harness change={change} />);
    await user.click(screen.getByRole("combobox", { name: "账号贡献人" }));
    const search = screen.getByRole("combobox", { name: "搜索贡献人" });
    expect(search).toHaveFocus();
    await user.type(search, query);
    const matches = screen.getAllByRole("option");
    expect(matches).toHaveLength(1);
    expect(matches[0]).toHaveTextContent(email);
    expect(change).not.toHaveBeenCalled();
  });

  it.each(["", "1"])("selects the right same-name user from initial value '%s' and restores focus", async (initial) => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<Harness change={change} initial={initial} />);
    await user.click(screen.getByRole("combobox", { name: "账号贡献人" }));
    await user.type(screen.getByRole("combobox", { name: "搜索贡献人" }), "wang");
    expect(screen.getAllByRole("option")).toHaveLength(2);
    await user.keyboard("{ArrowDown}{Enter}");
    expect(change).toHaveBeenCalledWith("2");
    expect(screen.getByRole("combobox", { name: "账号贡献人" })).toHaveTextContent("王明 · 运营部");
    expect(screen.getByRole("combobox", { name: "账号贡献人" })).toHaveFocus();
    expect(screen.getByRole("dialog", { name: "账号月记录" })).toBeVisible();
  });

  it("retains the selection on empty results and Escape, resetting search on reopen", async () => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<Harness change={change} />);
    await user.click(screen.getByRole("combobox", { name: "账号贡献人" }));
    await user.type(screen.getByRole("combobox", { name: "搜索贡献人" }), "no-such-user");
    expect(screen.getByText("没有匹配的用户")).toBeVisible();
    await user.keyboard("{Enter}{Escape}");
    expect(change).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "账号月记录" })).toBeVisible();
    await user.click(screen.getByRole("combobox", { name: "账号贡献人" }));
    expect(screen.getByRole("combobox", { name: "搜索贡献人" })).toHaveValue("");
    await user.click(screen.getByRole("option", { name: /李华/ }));
    expect(change).toHaveBeenCalledWith("3");
  });

  it("displays a saved contributor missing from available users and respects disabled", () => {
    render(<CostContributorSelect value="1" current={contributors[0]} contributors={[]} onChange={vi.fn()} disabled />);
    expect(screen.getByRole("combobox", { name: "账号贡献人" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "账号贡献人" })).toHaveTextContent("王明 · 研发部");
  });

  it("keeps the adjacent payment select usable after closing contributor search", async () => {
    const user = userEvent.setup();
    render(<Harness change={vi.fn()} />);
    await user.click(screen.getByRole("combobox", { name: "账号贡献人" }));
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("combobox", { name: "付费方式" }));
    expect(screen.getByRole("option", { name: "即用即付" })).toBeVisible();
    await user.keyboard("{Escape}");
    expect(screen.getByRole("combobox", { name: "付费方式" })).toHaveFocus();
    expect(screen.getByRole("dialog", { name: "账号月记录" })).toBeVisible();
  });
});
