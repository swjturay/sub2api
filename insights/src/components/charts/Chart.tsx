/* eslint-disable react-refresh/only-export-components */
import { useEffect, useMemo, useRef, useState } from "react";
import ReactECharts from "echarts-for-react";
import type { EChartsOption } from "echarts";
import { chartColors, chartFontFamily, palette, recolorChartOption } from "./chartTheme";
export function Chart({
  option,
  height = 280,
  label,
  onEvents,
}: {
  option: EChartsOption;
  height?: number;
  label: string;
  onEvents?: Record<string, (p: unknown) => void>;
}) {
  const [reducedMotion, setReducedMotion] = useState(() => typeof window !== "undefined" && typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  const [colors, setColors] = useState(chartColors);
  const optionRef = useRef(option);
  const optionColorsRef = useRef(colors);
  if (optionRef.current !== option) {
    optionRef.current = option;
    optionColorsRef.current = chartColors();
  }
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReducedMotion(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    const observer = new MutationObserver(() => setColors(chartColors()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "style"] });
    return () => observer.disconnect();
  }, []);
  const themedOption = useMemo<EChartsOption>(() => {
    const recolored = recolorChartOption(option, optionColorsRef.current, colors);
    return {
      backgroundColor: "transparent",
      textStyle: { color: colors.ink, fontFamily: chartFontFamily() },
      animation: !reducedMotion,
      animationDuration: reducedMotion ? 0 : 220,
      animationDurationUpdate: reducedMotion ? 0 : 180,
      animationEasingUpdate: "cubicOut",
      ...recolored,
    };
  }, [colors, option, reducedMotion]);
  return (
    <div role="img" aria-label={label} className="min-w-0 overflow-hidden">
      <ReactECharts
        option={themedOption}
        style={{ height }}
        notMerge
        lazyUpdate
        onEvents={onEvents}
      />
    </div>
  );
}
export { palette };
export const chartText = () => chartColors().muted;
