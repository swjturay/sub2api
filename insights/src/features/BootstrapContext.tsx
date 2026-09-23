/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, type ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { insightsApi } from "../lib/api";
import { useRemote } from "../lib/useRemote";
import type { Coverage, DepartmentOption, ModelOption } from "../lib/types";
import { Loading, ErrorBanner, CoverageBanner } from "../components/ui/States";
interface Bootstrap {
  models: ModelOption[];
  departments: DepartmentOption[];
  timezone: string;
  generatedAt: string;
  retention: { usageDays: number; errorDays: number };
  coverage: Coverage;
  modelCoverage: Coverage;
}
const Context = createContext<Bootstrap | null>(null);
export function BootstrapProvider({ children }: { children: ReactNode }) {
  const state = useRemote((s) => insightsApi.bootstrap(s), [], false);
  const location = useLocation();
  if (state.loading && !state.data)
    return (
      <main className="p-6">
        <Loading label="正在初始化看板" />
      </main>
    );
  if (!state.data)
    return (
      <main className="p-6">
        <ErrorBanner
          message={state.error || "初始化失败"}
          retry={state.refresh}
        />
      </main>
    );
  return (
    <Context.Provider value={state.data}>
      {state.error && (
        <div className="px-4 pt-3">
          <ErrorBanner message={state.error} retry={state.refresh} stale />
        </div>
      )}
      <CoverageBanner coverage={state.data.coverage} />
      {location.pathname.startsWith("/insights/models") && (
        <CoverageBanner coverage={state.data.modelCoverage} />
      )}
      {children}
    </Context.Provider>
  );
}
export const useBootstrap = () => {
  const value = useContext(Context);
  if (!value) throw new Error("BootstrapProvider missing");
  return value;
};
