import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { platformDateParts } from "../lib/format";
import type { FilterState } from "../lib/types";
import { AnalyticsFilters } from "./Filters";

vi.mock("./BootstrapContext", () => ({ useBootstrap: () => ({ timezone: "Asia/Tokyo" }) }));
afterEach(() => { cleanup(); vi.useRealTimers(); });

const value: FilterState = { from: "2026-09-14", to: "2026-09-20", granularity: "day", models: [], departments: [] };

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
