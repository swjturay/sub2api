/* eslint-disable @typescript-eslint/no-explicit-any */
import { apiFetch } from "./auth";
import type * as T from "./types";
type Meta = {
  timezone: string;
  generated_at: string;
  statistics_start_date?: string;
  coverage: Array<{
    dataset: string;
    status: string;
    detail?: string;
  }>;
};
type Wire<D> = { data: D; meta: Meta };
const q = (v: Record<string, string | number | undefined | string[]>) => {
  const p = new URLSearchParams();
  Object.entries(v).forEach(([k, x]) =>
    Array.isArray(x)
      ? x.forEach((y) => p.append(k, y))
      : x !== undefined && x !== "" && p.set(k, String(x)),
  );
  return p.size ? `?${p}` : "";
};
const fq = (f: T.FilterState) => ({
  from: f.from,
  to: f.to,
  granularity: f.granularity,
  model: f.models,
  department: f.departments,
});
const coverageLabels: Record<string, string> = {
  model_catalog: "模型目录",
  model_profile: "模型资料",
  usage_detail: "用量历史",
  error_detail: "错误历史",
  daily_user_model: "历史汇总",
  daily_rollups: "年度汇总",
  call_results: "网关调用",
  daily_call_results: "网关历史汇总",
  department_attribute: "部门字段",
  first_activity: "首次活跃记录",
};
const coverageMessage = (x: Meta["coverage"][number]) => {
  if (x.dataset === "department_attribute" && x.status === "not_configured") return "部门字段尚未配置";
  if (x.status === "partial") return (coverageLabels[x.dataset] || "部分数据") + "尚未完整采集";
  if (x.status === "not_collected") return `${coverageLabels[x.dataset] || "相关数据"}尚未采集`;
  return `${coverageLabels[x.dataset] || "相关数据"}暂不可用`;
};
const cov = (m: Meta, datasets?: string[]): T.Coverage => {
  const x = m.coverage.find((c) => (!datasets || datasets.includes(c.dataset)) && c.status !== "complete");
  return x
    ? {
        state:
          x.status === "partial"
            ? "partial"
            : x.status === "not_collected" || x.status === "not_configured"
              ? "uncollected"
              : "unavailable",
        message: coverageMessage(x),
      }
    : { state: "complete" };
};
export const adaptTokens = (x: any): T.TokenBreakdown => ({
  input: x?.input ?? null,
  cacheWrite: x?.cache_write ?? null,
  cacheRead: x?.cache_read ?? null,
  output: x?.output ?? null,
  total: x?.total ?? null,
});
export const adaptTimePoint = (x: any): T.TimePoint => ({
  bucket: x.start || x.at || x.date,
  totalTokens: x.metrics?.tokens?.total ?? x.tokens?.total ?? null,
  outputTokens: x.metrics?.tokens?.output ?? x.tokens?.output ?? null,
  cacheHitRate: x.metrics?.cache_hit_ratio ?? x.cache_hit_ratio ?? null,
  requests: x.metrics?.request_count ?? x.request_count ?? 0,
  successRate: x.quality?.success_rate ?? x.success_rate ?? null,
  users: x.total_users,
  incomplete: x.complete === false,
  models: Array.isArray(x.models) ? x.models.map(adaptModelSlice) : undefined,
});
export const adaptModelSlice = (x: any): T.ModelSlice => ({
  modelId:
    x.model?.identity ||
    (x.model?.platform && x.model?.name
      ? x.model.platform + ":" + x.model.name
      : String(x.model || "")),
  name: x.model?.name || x.model?.display_name || x.model || "",
  platform: x.model?.platform || "",
  requests: x.request_count || x.count || 0,
  totalTokens: x.tokens?.total ?? null,
  outputTokens: x.tokens?.output ?? null,
});

export function resolveCatalogModel(modelId: string, catalog: T.ModelOption[]) {
  const separator = modelId.indexOf(":");
  if (separator < 0 || modelId.slice(0, separator).toLowerCase() !== "unknown") return undefined;
  const modelName = modelId.slice(separator + 1).trim().toLowerCase();
  if (!modelName) return undefined;
  const matches = catalog.filter((option) => option.id.slice(option.id.indexOf(":") + 1).toLowerCase() === modelName);
  return matches.length === 1 ? matches[0] : undefined;
}

export function resolvePersonalAnalyticsModels(data: T.PersonalAnalytics, catalog: T.ModelOption[]): T.PersonalAnalytics {
  const resolveSlice = (slice: T.ModelSlice) => {
    const match = resolveCatalogModel(slice.modelId, catalog);
    return match ? { ...slice, modelId: match.id, name: match.name, platform: match.platform } : slice;
  };
  return {
    ...data,
    models: data.models.map(resolveSlice),
    series: data.series.map((point) => ({
      ...point,
      models: point.models?.map(resolveSlice),
    })),
  };
}

function previewModelSeries(series: T.TimePoint[], models: T.ModelSlice[]) {
  if (!import.meta.env.DEV || import.meta.env.VITE_INSIGHTS_PREVIEW_FIXTURES !== "true" || series.some((point) => point.models?.length) || !models.length) return series;
  const allocate = (total: number | null, metric: "totalTokens" | "outputTokens") => {
    if (total === null) return models.map(() => null);
    const weights = models.map((model) => Math.max(0, model[metric] ?? model.requests));
    const weightTotal = weights.reduce((sum, value) => sum + value, 0);
    if (weightTotal <= 0) return models.map(() => 0);
    let assigned = 0;
    return weights.map((weight, index) => {
      if (index === weights.length - 1) return Math.max(0, total - assigned);
      const value = Math.round(total * weight / weightTotal);
      assigned += value;
      return value;
    });
  };
  return series.map((point) => {
    const totalTokens = allocate(point.totalTokens, "totalTokens");
    const outputTokens = allocate(point.outputTokens, "outputTokens");
    return {
      ...point,
      models: models.map((model, index) => ({ ...model, totalTokens: totalTokens[index], outputTokens: outputTokens[index] })),
    };
  });
}

function previewSubscriptionUsage(id: unknown, generatedAt: string): T.SubscriptionUsagePoint[] {
  const end = new Date(generatedAt);
  end.setUTCSeconds(0, 0);
  const seed = String(id).split("").reduce((total, char) => total + char.charCodeAt(0), 0);
  return Array.from({ length: 60 }, (_, index) => {
    const at = new Date(end.getTime() - (59 - index) * 60_000);
    const wave = Math.sin((index + seed) / 5) + Math.sin((index + seed * 2) / 11);
    const active = (index * 7 + seed) % 13 < 8;
    const requests = active ? 1 + ((index * 3 + seed) % 9) : 0;
    const amount = requests === 0 ? 0 : Number((requests * (0.0018 + (wave + 2) * 0.0011)).toFixed(6));
    return { at: at.toISOString(), amount, requests };
  });
}
const mv = (
  value: number | null | undefined,
  sampleCount?: number,
): T.MetricValue => ({ value: value ?? null, sampleCount });
export const adaptModelProfile = (x: any): T.ModelProfile => ({
  id: `${x.identity.platform}:${x.identity.name}`,
  name: x.identity.display_name || x.identity.name,
  platform: x.identity.platform,
  description: x.profile?.description ?? null,
  useCases: x.profile?.use_cases ?? [],
  contextLimit: x.profile?.context_limit ?? null,
  maxOutput: x.profile?.max_output ?? null,
  inputModalities: x.profile?.input_modalities || [],
  outputModalities: x.profile?.output_modalities || [],
  reasoning: x.profile?.reasoning ?? "unknown",
  toolCalling: x.profile?.tool_calling ?? "unknown",
  structuredOutput: x.profile?.structured_output ?? "unknown",
  sources: (x.profile?.sources || []).map((s: any) => ({ label: s.label, url: s.url, updatedAt: s.updated_at ?? null })),
  updatedAt: x.profile?.updated_at ?? null,
  version: x.profile?.version ?? 0,
  pricing: (x.reference_pricing?.items || []).map((p: any) => ({ label: p.kind, value: `${p.price} ${p.currency}/${p.unit}`, condition: p.condition })),
  metrics: {
    tpm: mv(x.performance?.average_tpm, x.performance?.tpm_samples),
    rpm: mv(x.performance?.average_rpm, x.performance?.rpm_samples),
    ttft: mv(x.performance?.average_ttft_ms, x.performance?.ttft_samples),
    tpot: mv(x.performance?.estimated_tpot_ms, x.performance?.tpot_samples),
  },
});

export const insightsApi = {
  bootstrap: async (signal?: AbortSignal) => {
    const r = await apiFetch<Wire<{ items: any[] }>>(
        "/insights/models?window=24h",
        {},
        signal,
      ),
      u = JSON.parse(localStorage.getItem("auth_user") || "{}");
    let departments: T.DepartmentOption[] = [];
    let retention = { usageDays: 365, errorDays: 30 };
    if (u.is_admin || u.role === "admin") {
      const d = await apiFetch<Wire<any>>(
        "/admin/insights/dimensions",
        {},
        signal,
      );
      departments = (d.data.department?.options || []).map((x: any) => ({
        id: x.value,
        name: x.label,
      }));
      retention = {
        usageDays: d.data.retention?.usage_detail_days || 365,
        errorDays: d.data.retention?.error_detail_days || 30,
      };
    }
    return {
      models: r.data.items.map((x) => ({
        id: `${x.identity.platform}:${x.identity.name}`,
        name: x.identity.display_name || x.identity.name,
        platform: x.identity.platform,
      })),
      departments,
      timezone: r.meta.timezone,
      generatedAt: r.meta.generated_at,
      retention,
      coverage: cov(r.meta, ["model_catalog"]),
      modelCoverage: cov(r.meta, ["model_profile"]),
    };
  },
  personalOverview: async (signal?: AbortSignal) => {
    const r = await apiFetch<Wire<any>>("/insights/me/today", {}, signal);
    const previewEnabled = import.meta.env.DEV && import.meta.env.VITE_INSIGHTS_PREVIEW_FIXTURES === "true";
    return {
      subscriptions: (r.data.subscriptions || []).map((s: any) => {
        const hasRecentUsage = Array.isArray(s.recent_usage);
        return {
          id: String(s.id),
          name: s.name,
          used: s.used_amount,
          limit: s.limit_amount,
          currency: s.currency || "",
          remaining: s.remaining_amount,
          resetsAt: s.reset_at,
          status: s.unlimited
            ? "unlimited"
            : s.over_limit
              ? "exceeded"
              : "active",
          recentUsage: hasRecentUsage
            ? s.recent_usage.map((point: any) => ({ at: point.at, amount: Number(point.amount ?? 0), requests: Number(point.requests ?? 0) }))
            : previewEnabled
              ? previewSubscriptionUsage(s.id, r.meta.generated_at)
              : [],
          recentUsagePreview: !hasRecentUsage && previewEnabled,
        };
      }),
      todayTokens: adaptTokens(r.data.tokens),
      totalAmount: Number(r.data.actual_cost ?? 0),
      timezone: r.meta.timezone,
      coverage: cov(r.meta),
    } as T.PersonalOverview;
  },
  personalHeatmap: async (year: number, signal?: AbortSignal): Promise<T.PersonalHeatmap> => {
    const r = await apiFetch<Wire<any>>(
      `/insights/me/heatmap${q({ year })}`,
      {},
      signal,
    );
    if (!Array.isArray(r.data?.days))
      throw new Error("热力图响应缺少日期数据");
    if (!Array.isArray(r.data?.scale?.thresholds))
      throw new Error("热力图响应缺少跨年份共享色阶");
    return {
      days: r.data.days.map((d: any) => ({
        date: d.date,
        tokens: d.total_tokens ?? null,
        state: d.state as T.HeatmapDay["state"],
      })),
      thresholds: r.data.scale.thresholds,
      statisticsStartDate: r.meta.statistics_start_date,
      coverage: cov(r.meta),
    };
  },
  personalAnalytics: async (f: T.FilterState, signal?: AbortSignal) => {
    const r = await apiFetch<Wire<any>>(
        `/insights/me/usage${q(fq(f))}`,
        {},
        signal,
      ),
      s = r.data.summary,
      models = r.data.models.map(adaptModelSlice),
      series = previewModelSeries(r.data.buckets.map(adaptTimePoint), models);
    return {
      activeDays: s.active_days,
      totalTokens: s.tokens.total,
      averageDailyTokens: s.daily_average_tokens,
      requests: s.request_count,
      outputTokens: s.tokens.output,
      cacheHitRate: s.cache_hit_ratio,
      series,
      models,
      coverage: cov(r.meta),
    } as T.PersonalAnalytics;
  },
  logs: async (
    kind: "usage" | "errors",
    f: T.FilterState,
    cursor: string | undefined,
    pageSize: number,
    signal?: AbortSignal,
  ) => {
    const r = await apiFetch<Wire<any>>(
      `/insights/me/${kind === "usage" ? "usage-logs" : "errors"}${q({ ...fq(f), cursor, page_size: pageSize })}`,
      {},
      signal,
    );
    return {
      items: r.data.items.map((x: any) =>
        kind === "usage"
          ? {
              id: x.id,
              recordedAt: x.recorded_at,
              department: x.department,
              model: x.model,
              apiKeyName: x.api_key_name,
              inputTokens: x.tokens.input,
              outputTokens: x.tokens.output,
              cacheReadTokens: x.tokens.cache_read,
              cacheWriteTokens: x.tokens.cache_write,
              durationMs: x.duration_ms,
              ttftMs: x.first_token_ms,
              metadata: x.metadata || {},
            }
          : {
              id: x.id,
              recordedAt: x.recorded_at,
              model: x.model,
              type: x.error_type,
              reason: x.reason,
              metadata: x.metadata || {},
            },
      ),
      nextCursor: r.data.page?.next_cursor,
      hasMore: r.data.page?.has_more || false,
      coverage: cov(r.meta),
    };
  },
  models: async (window: string, signal?: AbortSignal) => {
    const r = await apiFetch<Wire<{ items: any[] }>>(
      `/insights/models${q({ window })}`,
      {},
      signal,
    );
    return r.data.items.map(adaptModelProfile);
  },
  model: async (id: string, window: string, signal?: AbortSignal) => {
    const r = await apiFetch<Wire<any>>(
      `/insights/models/detail${q({ model: id, window })}`,
      {},
      signal,
    );
    return adaptModelProfile(r.data);
  },
  compareModels: async (models: string[], window: string, signal?: AbortSignal) => {
    const r = await apiFetch<Wire<any>>(`/insights/models/compare${q({ model: models, window })}`, {}, signal);
    return {
      models: (r.data.models || []).map(adaptModelProfile),
      trends: (r.data.trends || []).map((t: any) => ({
        at: t.at,
        complete: t.complete !== false,
        values: (t.values || []).map((v: any) => ({
          model: typeof v.model === "string" ? v.model : `${v.model.platform}:${v.model.name}`,
          performance: {
            tpm: v.performance?.average_tpm ?? null,
            rpm: v.performance?.average_rpm ?? null,
            ttft: v.performance?.average_ttft_ms ?? null,
            tpot: v.performance?.estimated_tpot_ms ?? null,
          },
        })),
      })),
    } as T.ModelComparison;
  },
  departments: async (
    f: T.FilterState,
    performanceModel: string,
    signal?: AbortSignal,
  ) => {
    const r = await apiFetch<Wire<any>>(
        `/admin/insights/departments${q({ ...fq(f), performance_model: performanceModel || undefined })}`,
        {},
        signal,
      ),
      s = r.data.summary;
    if (!Array.isArray(r.data.pareto?.department_items) || !Array.isArray(r.data.pareto?.member_items))
      throw new Error("部门帕累托响应缺少完整部门或成员列表");
    return {
      members: s.member_count,
      activeMembers: s.active_member_count,
      totalTokens: s.tokens.total,
      averageDailyTokens: s.daily_average_tokens,
      perMemberDailyTokens: s.daily_per_member_tokens,
      requests: s.request_count,
      outputTokens: s.tokens.output,
      outputShare: s.output_ratio,
      cacheHitRate: s.cache_hit_ratio,
      series: r.data.buckets.map(adaptTimePoint),
      models: r.data.models.map(adaptModelSlice),
      departmentPareto: (r.data.pareto?.department_items || []).map((x: any) => ({
        id: x.id,
        name: x.label,
        tokens: x.total_tokens,
        cumulativeShare: x.cumulative_ratio,
      })),
      memberPareto: (r.data.pareto?.member_items || []).map((x: any) => ({
        id: x.id,
        name: x.label,
        tokens: x.total_tokens,
        cumulativeShare: x.cumulative_ratio,
      })),
      topUsers: (r.data.top_users || []).map((x: any) => ({
        id: x.user_id,
        name: x.username,
        tokens: x.total_tokens,
      })),
      performance: {
        tpm: mv(r.data.performance?.average_tpm),
        rpm: mv(r.data.performance?.average_rpm),
        ttft: mv(
          r.data.performance?.average_ttft_ms,
          r.data.performance?.ttft_samples,
        ),
        tpot: mv(
          r.data.performance?.estimated_tpot_ms,
          r.data.performance?.tpot_samples,
        ),
      },
      coverage: cov(r.meta),
    } as T.DepartmentAnalytics;
  },
  gateway: async (f: T.FilterState, signal?: AbortSignal) => {
    const query = q({ ...fq(f), model: undefined }),
      [quality, users, retention, prefs] = await Promise.all([
        apiFetch<Wire<any>>(
          `/admin/insights/gateway/quality${query}`,
          {},
          signal,
        ),
        apiFetch<Wire<any>>(
          `/admin/insights/gateway/users${query}`,
          {},
          signal,
        ),
        apiFetch<Wire<any>>(
          `/admin/insights/gateway/retention${q({ to: f.to, department: f.departments })}`,
          {},
          signal,
        ),
        apiFetch<Wire<any>>(
          `/admin/insights/gateway/model-preferences${query}`,
          {},
          signal,
        ),
      ]),
      s = quality.data.summary,
      fr = users.data.frequency,
      layer = (key: string, label: string) => ({
        key,
        label,
        count: retention.data[key].count,
        share: retention.data[key].ratio,
        status: retention.data[key].status,
      });
    return {
      totalCalls: s.total,
      failedCalls: s.failed,
      successRate: s.success_rate,
      modelDuration: mv(s.model_duration_ms),
      gatewayDuration: mv(s.gateway_duration_ms),
      series: quality.data.buckets.map(adaptTimePoint),
      totalUsers: users.data.summary.end_users,
      newUsers: users.data.summary.new_users,
      frequency: {
        high: fr.high,
        medium: fr.medium,
        low: fr.low,
                unclassified: fr.unclassified ?? 0,
coverage: cov(users.meta),
      },
      userSeries: users.data.buckets.map(adaptTimePoint),
      funnel: [
        layer("total_users", "总用户数"),
        layer("first_request", "首次请求"),
        layer("next_day", "次日留存"),
        layer("day_7", "7日留存"),
        layer("day_30", "30日留存"),
      ],
      preferences: prefs.data.departments.map((d: any) => ({
        department: d.label,
        total: d.total,
        models: d.models.map((m: any) => ({
          model: m.model,
          count: m.count,
          share: m.ratio ?? null,
        })),
      })),
      coverage: cov(quality.meta),
          retentionCoverage: cov(retention.meta),
      preferenceCoverage: cov(prefs.meta),
    } as T.GatewayAnalytics;
  },
  costs: async (filters: T.CostFilters, signal?: AbortSignal): Promise<T.CostDashboard> => {
    const r = await apiFetch<Wire<any>>(`/admin/insights/costs${q({
      month: filters.month,
      department: filters.department,
      platform: filters.platform,
      q: filters.q,
      contributor: filters.contributor,
      payment_method: filters.paymentMethod,
      status: filters.status,
      completeness: filters.completeness,
      registration: filters.registration,
      page: filters.page,
      page_size: filters.pageSize,
    })}`, {}, signal);
    const contributor = (value: any): T.CostContributor | undefined => value ? ({
      id: value.id,
      name: value.name,
      email: value.email,
      status: value.status,
      departmentId: value.department_id,
      department: value.department,
      deleted: value.deleted,
    }) : undefined;
    const money = (value: unknown) => Number(value ?? 0);
    const s = r.data.summary;
    return {
      month: r.data.month,
      inProgress: Boolean(r.data.in_progress),
      summary: {
        contributorCount: s.contributor_count,
        accountCount: s.account_count,
        platformCost: money(s.platform_cost),
        actualCost: money(s.actual_cost),
        savings: money(s.savings),
        completedAccounts: s.completed_accounts,
        totalAccounts: s.total_accounts,
      },
      trend: (r.data.trend || []).map((x: any) => ({ month: x.month, platformCost: money(x.platform_cost), actualCost: money(x.actual_cost), savings: money(x.savings), completedAccounts: x.completed_accounts, totalAccounts: x.total_accounts })),
      contributionDepartments: (r.data.contribution_departments || []).map((x: any) => ({ id: x.id, name: x.name, contributorCount: x.contributor_count, accountCount: x.account_count, requestCount: x.request_count, tokens: x.tokens, platformCost: money(x.platform_cost), actualCost: money(x.actual_cost), savings: money(x.savings), completedAccounts: x.completed_accounts, totalAccounts: x.total_accounts })),
      usageDepartments: (r.data.usage_departments || []).map((x: any) => ({ id: x.id, name: x.name, requestCount: x.request_count, tokens: x.tokens, platformCost: money(x.platform_cost) })),
      flows: (r.data.flows || []).map((x: any) => ({ contributionDepartmentId: x.contribution_department_id, contributionDepartment: x.contribution_department, usageDepartmentId: x.usage_department_id, usageDepartment: x.usage_department, platformCost: money(x.platform_cost) })),
      accounts: {
        items: (r.data.accounts?.items || []).map((x: any) => ({
          id: x.id, name: x.name, platform: x.platform, type: x.type, status: x.status, expiresAt: x.expires_at, deletedAt: x.deleted_at,
          registered: Boolean(x.registered), inherited: Boolean(x.inherited), configurationMonth: x.configuration_month,
          contributor: contributor(x.contributor), paymentMethod: x.payment_method || undefined, requestCount: x.request_count,
          tokens: adaptTokens(x.tokens), platformCost: money(x.platform_cost), actualCost: x.actual_cost === null ? null : money(x.actual_cost),
          savings: x.savings === null ? null : money(x.savings), notes: x.notes || "", updatedBy: contributor(x.updated_by), updatedAt: x.updated_at,
        })),
        total: r.data.accounts?.total || 0,
        page: r.data.accounts?.page || 1,
        pageSize: r.data.accounts?.page_size || filters.pageSize,
        pages: r.data.accounts?.pages || 1,
      },
      dimensions: {
        contributors: (r.data.dimensions?.contributors || []).map(contributor).filter(Boolean) as T.CostContributor[],
        departments: (r.data.dimensions?.departments || []).map((x: any) => ({ id: x.value, name: x.label })),
        platforms: r.data.dimensions?.platforms || [],
        statuses: r.data.dimensions?.statuses || [],
        paymentMethods: (r.data.dimensions?.payment_methods || []).map((x: any) => ({ id: x.value, name: x.label })),
      },
      coverage: cov(r.meta),
    };
  },
  saveCostMonth: (accountId: number, month: string, input: { contributor_user_id: number; payment_method: T.CostPaymentMethod; actual_cost: string | null; notes: string }) =>
    apiFetch<{ saved: boolean }>(`/admin/insights/costs/accounts/${accountId}/months/${month}`, { method: "PUT", body: JSON.stringify(input) }),
  stopCostAccount: (accountId: number, afterMonth: string) =>
    apiFetch<{ stopped_after: string }>(`/admin/insights/costs/accounts/${accountId}/stop`, { method: "POST", body: JSON.stringify({ after_month: afterMonth }) }),
};
