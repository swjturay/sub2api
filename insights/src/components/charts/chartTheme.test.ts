import { describe, expect, it } from "vitest";
import { compactNumber, fullNumber, recolorChartOption, tooltipTheme, type ChartColors } from "./chartTheme";

describe("chart theme", () => {
  it("recolors nested canvas options without changing semantic palette colors", () => {
    const light: ChartColors = { surface: "#fff", surfaceSubtle: "#f5f7fa", border: "#ddd", ink: "#111", muted: "#666", primary: "#1769e0" };
    const dark: ChartColors = { surface: "#111824", surfaceSubtle: "#182231", border: "#2a394d", ink: "#edf3fa", muted: "#aebbd0", primary: "#75a7ff" };
    const option = { tooltip: { backgroundColor: light.surface }, calendar: [{ itemStyle: { borderColor: light.surface } }], color: ["#2878e6"] };
    const result = recolorChartOption(option, light, dark);
    expect(result).toEqual({ tooltip: { backgroundColor: dark.surface }, calendar: [{ itemStyle: { borderColor: dark.surface } }], color: ["#2878e6"] });
    expect(option.tooltip.backgroundColor).toBe(light.surface);
  });
  it("preserves small non-zero values and distinguishes unavailable from zero", () => {
    expect(compactNumber(0.00468)).toBe("0.00468");
    expect(compactNumber(0.000001)).toBe("0.000001");
    expect(compactNumber(null)).toBe("—");
    expect(compactNumber(0)).toBe("0");
    expect(fullNumber(100000)).toBe("100,000");
  });
  it("confines wrapping tooltips to the chart viewport", () => {
    const tooltip = tooltipTheme();
    expect(tooltip.confine).toBe(true);
    expect(tooltip.extraCssText).toContain("max-width:min(320px,calc(100vw - 32px))");
    expect(tooltip.extraCssText).toContain("overflow-wrap:anywhere");
  });
});
