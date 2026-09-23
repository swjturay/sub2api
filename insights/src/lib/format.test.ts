import { describe, expect, it } from "vitest";
import {
  metric,
  money,
  dateTime,
  percent,
  platformDateParts,
  subtractPlatformDays,
} from "./format";
describe("analytics formatting", () => {
  it("keeps unavailable ratios distinct from zero", () => {
    expect(percent(null)).toBe("—");
    expect(percent(0)).toBe("0.0%");
    expect(percent(0.000001)).toBe("0.0001%");
  });
  it("does not render a non-zero small metric as zero", () => {
    expect(metric(0.4)).toBe("0.4");
    expect(metric(0.02718)).toBe("0.0272");
    expect(metric(0.00468)).toBe("0.00468");
    expect(metric(0.0001)).toBe("0.0001");
  });
});
describe("platform calendar defaults", () => {
  it("uses Shanghai date across Tokyo midnight", () => {
    expect(platformDateParts("2026-09-22T15:30:00Z", "Asia/Shanghai")).toEqual({
      date: "2026-09-22",
      year: 2026,
    });
    expect(platformDateParts("2026-09-22T15:30:00Z", "Asia/Tokyo").date).toBe(
      "2026-09-23",
    );
  });
  it("keeps the platform year before Shanghai new year", () => {
    expect(
      platformDateParts("2026-12-31T15:30:00Z", "Asia/Shanghai").year,
    ).toBe(2026);
    expect(subtractPlatformDays("2027-01-01", 6)).toBe("2026-12-26");
  });
});

describe("platform timestamp formatting",()=>{
  it("uses the configured Shanghai timezone across Tokyo midnight",()=>{
    expect(dateTime("2026-09-22T15:30:00Z","Asia/Shanghai")).toContain("2026年9月22日");
    expect(dateTime("2026-09-22T15:30:00Z","Asia/Tokyo")).toContain("2026年9月23日");
  });
});
describe("ledger money formatting",()=>{
  it("keeps regular and micro amounts without turning a positive amount into zero",()=>{
    expect(money(42.8)).toBe("42.80");
    expect(money(0.0001)).toBe("0.0001");
    expect(money(0)).toBe("0.00");
  });
});