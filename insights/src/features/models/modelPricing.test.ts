import { describe, expect, it } from "vitest";
import { formatPricingEntry } from "./modelPricing";

describe("model pricing presentation", () => {
  it("converts finite USD per-token values and labels known tier conditions", () => {
    expect(formatPricingEntry({
      label: "input",
      value: "0.000005 USD/token",
      condition: "min_tokens=0,max_tokens=272000,tier=≤272K",
    })).toEqual({
      label: "输入",
      displayValue: "5 USD/百万 Token",
      rawValue: "0.000005 USD/token",
      rawCondition: "min_tokens=0,max_tokens=272000,tier=≤272K",
      conditions: [
        { label: "起始 Token", value: "0" },
        { label: "截止 Token", value: "272,000" },
        { label: "价格档位", value: "≤272K" },
      ],
    });
  });

  it("preserves zero and tiny recognized prices", () => {
    expect(formatPricingEntry({ label: "output", value: "0 USD/token" }).displayValue)
      .toBe("0 USD/百万 Token");
    expect(formatPricingEntry({ label: "output", value: "0.000000000001 USD/token" }).displayValue)
      .toBe("0.000001 USD/百万 Token");
  });

  it("leaves unknown units and condition syntax unchanged", () => {
    expect(formatPricingEntry({
      label: "custom",
      value: "2 USD/request",
      condition: "regional pricing",
    })).toEqual({
      label: "custom",
      displayValue: "2 USD/request",
      rawValue: "2 USD/request",
      rawCondition: "regional pricing",
      conditions: [{ label: "条件", value: "regional pricing" }],
    });
    expect(formatPricingEntry({
      label: "custom",
      value: "2 credits/unit",
      condition: "region=apac",
    }).conditions).toEqual([{ label: "条件", value: "region=apac" }]);
  });
});
