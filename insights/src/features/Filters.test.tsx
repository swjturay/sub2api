import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { platformDateParts } from "../lib/format";
import type { FilterState } from "../lib/types";
import { AnalyticsFilters, MonthFilter } from "./Filters";

vi.mock("./BootstrapContext", () => ({ useBootstrap: () => ({ timezone: "Asia/Tokyo" }) }));
afterEach(() => { cleanup(); vi.useRealTimers(); });

const value: FilterState = { from: "2026-09-14", to: "2026-09-20", granularity: "day", models: [], departments: [] };

describe("MonthFilter", () => {
  it("limits selection to available months and closes after selecting", async () => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<MonthFilter value="2026-09" min="2026-06" max="2026-09" onChange={change} />);
    await user.click(screen.getByRole("button", { name: "月份" }));
    expect(screen.getByRole("button", { name: "2026年9月" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "2026年5月" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "2026年10月" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "上一年" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "下一年" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "2026年6月" }));
    expect(change).toHaveBeenCalledWith("2026-06");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "月份" })).toHaveFocus();
  });

  it("navigates years, cancels with Escape and reopens at the selected month", async () => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<MonthFilter value="2026-12" min="2026-06" max="2027-02" onChange={change} />);
    await user.click(screen.getByRole("button", { name: "月份" }));
    await user.click(screen.getByRole("button", { name: "下一年" }));
    expect(screen.getByRole("button", { name: "2027年3月" })).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(change).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "月份" }));
    expect(screen.getByRole("button", { name: "2026年12月" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "下一年" }));
    await user.click(screen.getByRole("button", { name: "2027年1月" }));
    expect(change).toHaveBeenCalledWith("2027-01");
  });
});

describe("AnalyticsFilters", () => {
  it("commits only a complete valid typed date range", async () => {
    const change = vi.fn();
    const user = userEvent.setup();
    render(<AnalyticsFilters value={value} onChange={change} models={[]} showModels={false} />);
    await user.click(screen.getByRole("button", { name: "日期范围" }));
    const start = screen.getByLabelText("开始日期");
    await user.clear(start);
    await user.type(start, "invalid");
    expect(screen.getByRole("button", { name: "应用" })).toBeDisabled();
    expect(change).not.toHaveBeenCalled();
    await user.clear(start);
    await user.type(start, "2026-09-10");
    await user.click(screen.getByRole("button", { name: "应用" }));
    expect(change).toHaveBeenCalledWith({ ...value, from: "2026-09-10", to: "2026-09-20" });
  });
  it("keeps the public state shape when changing granularity", async () => {
    const change = vi.fn();
    render(<AnalyticsFilters value={value} onChange={change} models={[]} showModels={false} />);
    await userEvent.click(within(screen.getByRole("group", { name: "统计粒度" })).getByRole("button", { name: "周" }));
    expect(change).toHaveBeenCalledWith({ ...value, granularity: "week" });
  });
  it("anchors presets to the current platform day", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-09-22T15:30:00.000Z"));
    const change = vi.fn();
    const user = userEvent.setup();
    const today = platformDateParts(new Date().toISOString(), "Asia/Tokyo").date;
    expect(today).toBe("2026-09-23");
    render(<AnalyticsFilters value={value} onChange={change} models={[]} showModels={false} />);
    await user.click(screen.getByRole("button", { name: "日期范围" }));
    await user.click(screen.getByRole("button", { name: "今天" }));
    expect(change).toHaveBeenCalledWith({ ...value, from: today, to: today });
  });
});
