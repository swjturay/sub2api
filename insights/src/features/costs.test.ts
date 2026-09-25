import { describe, expect, it } from "vitest";
import { costCompletionLabel, costSavingsPreview } from "./costs";

describe("cost data helpers", () => {
  it("keeps missing actual cost distinct from confirmed zero", () => {
    expect(costSavingsPreview({ platformCost: 10 }, "")).toBeNull();
    expect(costSavingsPreview({ platformCost: 10 }, "0.00")).toBe(10);
  });
  it("allows negative savings and formats completeness as a fraction", () => {
    expect(costSavingsPreview({ platformCost: 10 }, "12.00")).toBe(-2);
    expect(costCompletionLabel(3, 5)).toBe("3/5");
  });
});
