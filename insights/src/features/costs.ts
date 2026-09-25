import type { CostAccount } from "../lib/types";

export const costPaymentLabels = { subscription: "订阅", payg: "即用即付", other: "其他" } as const;

export function costCompletionLabel(completed: number, total: number) {
  return `${completed}/${total}`;
}

export function costSavingsPreview(account: Pick<CostAccount, "platformCost">, actualCost: string) {
  if (actualCost.trim() === "") return null;
  const actual = Number(actualCost);
  return Number.isFinite(actual) && actual >= 0 ? account.platformCost - actual : null;
}
