export const integer = new Intl.NumberFormat("zh-CN", {
  maximumFractionDigits: 0,
});
export const compact = new Intl.NumberFormat("zh-CN", {
  notation: "compact",
  maximumFractionDigits: 1,
});
const preciseSmall = new Intl.NumberFormat("zh-CN", { maximumSignificantDigits: 3 });
export const percent = (v: number | null, digits = 1) => {
  if (v === null) return "—";
  const value = v * 100;
  return `${value !== 0 && Math.abs(value) < 0.1 ? preciseSmall.format(value) : value.toFixed(digits)}%`;
};
export const metric = (v: number | null, unit = "") =>
  v === null
    ? "—"
    : `${v !== 0 && Math.abs(v) < 1 ? preciseSmall.format(v) : compact.format(v)}${unit}`;
export const money = (v: number) =>
  new Intl.NumberFormat("zh-CN", {
    minimumFractionDigits: 2,
    maximumFractionDigits: v !== 0 && Math.abs(v) < 0.01 ? 8 : 2,
  }).format(v);
export const exact = (v: number) => integer.format(v);
export const dateTime = (v: string | Date, timeZone: string) =>
  new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "medium",
    timeZone,
  }).format(new Date(v));
export const today = () => new Date().toISOString().slice(0, 10);
export const daysAgo = (n: number) => {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
};
export function platformDateParts(instant: string, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date(instant));
  const value = Object.fromEntries(parts.map((p) => [p.type, p.value]));
  return {
    date: `${value.year}-${value.month}-${value.day}`,
    year: Number(value.year),
  };
}
export function subtractPlatformDays(date: string, days: number) {
  const [year, month, day] = date.split("-").map(Number);
  const value = new Date(Date.UTC(year, month - 1, day - days));
  return value.toISOString().slice(0, 10);
}
