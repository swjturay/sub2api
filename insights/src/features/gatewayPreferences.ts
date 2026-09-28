import { resolveCatalogModel } from "../lib/api";
import type { GatewayAnalytics, ModelOption } from "../lib/types";

// Old call facts can retain the routing group instead of a concrete provider.
export function resolveGatewayPreferences(preferences: GatewayAnalytics["preferences"], catalog: ModelOption[]) {
  return preferences.map((department) => {
    const models = new Map<string, number>();
    for (const item of department.models) {
      const separator = item.model.indexOf(":");
      const provider = item.model.slice(0, separator).trim().toLowerCase();
      const name = item.model.slice(separator + 1).trim() || "unknown";
      const identity = !provider || provider === "composite" ? `unknown:${name}` : item.model;
      const resolved = resolveCatalogModel(identity, catalog)?.id || identity;
      models.set(resolved, (models.get(resolved) || 0) + item.count);
    }
    return { ...department, models: [...models].map(([model, count]) => ({
      model, count, share: department.total > 0 ? count / department.total : null,
    })) };
  });
}
