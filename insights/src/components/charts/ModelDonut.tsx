import type { ModelSlice } from "../../lib/types";
import { Chart, palette } from "./Chart";
import { chartColors, tooltipTheme } from "./chartTheme";
import { percent } from "../../lib/format";
export function ModelDonut({ models }: { models: ModelSlice[] }) {
  const total = models.reduce((s, m) => s + m.requests, 0),
    sorted = [...models].sort((a, b) => b.requests - a.requests);
  const chartData =
    sorted.length > 5
      ? [
          ...sorted
            .slice(0, 5)
            .map((m) => ({ name: m.name, value: m.requests })),
          {
            name: "其他",
            value: sorted.slice(5).reduce((s, m) => s + m.requests, 0),
          },
        ]
      : sorted.map((m) => ({ name: m.name, value: m.requests }));
  const colors = chartColors();
  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(280px,42%)_1fr]">
      <Chart
        label="模型调用分布"
        height={320}
        option={{
          color: palette,
          tooltip: { ...tooltipTheme(), trigger: "item", formatter: "{b}<br/><b>{c}</b> 次 · {d}%" },
          legend: { show: false },
          series: [
            {
              type: "pie",
              radius: ["52%", "76%"],
              center: ["50%", "48%"],
              label: { show: false },
              itemStyle: { borderColor: colors.surface, borderWidth: 3, borderRadius: 5 },
              emphasis: { scaleSize: 4, itemStyle: { shadowBlur: 8, shadowColor: "rgba(15,23,42,.16)" } },
              data: chartData,
            },
          ],
        }}
      />
      <div className="max-h-80 overflow-auto scrollbar-thin">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-[var(--surface)] text-left muted">
            <tr>
              <th className="py-2">模型</th>
              <th>平台</th>
              <th className="text-right">请求</th>
              <th className="text-right">占比</th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((m, i) => (
              <tr key={m.modelId} className="border-t border-[var(--border)]">
                <td className="py-2">
                  <span
                    className="mr-2 inline-block h-2.5 w-2.5 rounded-sm"
                    style={{
                      background:
                        sorted.length > 5 && i >= 5
                          ? palette[5]
                          : palette[i % palette.length],
                    }}
                  />
                  {m.name}
                </td>
                <td className="muted">{m.platform}</td>
                <td className="text-right tabular-nums">
                  {m.requests.toLocaleString()}
                </td>
                <td className="text-right tabular-nums">
                  {percent(total ? m.requests / total : null)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
