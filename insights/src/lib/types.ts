export type CoverageState = "complete" | "partial" | "uncollected" | "unavailable";
export type Granularity = "hour" | "day" | "week" | "month";
export type ChartMode = "line" | "bar";
export type Capability = "supported" | "unsupported" | "unknown";
export interface User { id:number; username?:string; email?:string; role?:string; is_admin?:boolean }
export interface ApiEnvelope<T> { code:number; message?:string; data:T }
export interface Coverage { state:CoverageState; message?:string }
export interface TokenBreakdown { input:number|null; cacheWrite:number|null; cacheRead:number|null; output:number|null; total:number|null }
export interface MetricValue { value:number|null; unit?:string; sampleCount?:number; coverage?:Coverage }
export interface ModelSlice { modelId:string; name:string; platform:string; requests:number; totalTokens:number|null; outputTokens:number|null }
export interface TimePoint { bucket:string; totalTokens:number|null; outputTokens:number|null; cacheHitRate:number|null; requests:number; successRate?:number|null; users?:number; incomplete?:boolean; models?:ModelSlice[] }
export interface ModelOption { id:string; name:string; platform:string }
export interface DepartmentOption { id:string; name:string; issue?:string }
export interface FilterState { from:string; to:string; granularity:Granularity; models:string[]; departments:string[] }
export interface SubscriptionUsagePoint { at:string; amount:number; requests:number }
export interface SubscriptionUsage { id:string; name:string; used:number; limit:number|null; currency:string; remaining:number|null; resetsAt:string|null; status:"active"|"unlimited"|"exceeded"; recentUsage:SubscriptionUsagePoint[]; recentUsagePreview:boolean }
export interface PersonalOverview { subscriptions:SubscriptionUsage[]; todayTokens:TokenBreakdown; totalAmount:number; timezone:string; coverage:Coverage }
export interface HeatmapDay { date:string; tokens:number|null; state:"value"|"zero"|"out_of_scope"|"future" }
export interface PersonalHeatmap { days:HeatmapDay[]; thresholds:number[]; statisticsStartDate?:string; coverage:Coverage }
export interface PersonalAnalytics { activeDays:number; totalTokens:number|null; averageDailyTokens:number; requests:number; outputTokens:number|null; cacheHitRate:number|null; series:TimePoint[]; models:ModelSlice[]; coverage:Coverage }
export interface UsageLog { id:string; recordedAt:string; department:string; model:string; apiKeyName:string; inputTokens:number; outputTokens:number; cacheReadTokens:number; cacheWriteTokens:number; durationMs:number|null; ttftMs:number|null; metadata:Record<string,unknown> }
export interface ErrorLog { id:string; recordedAt:string; model:string; type:string; reason:string; metadata:Record<string,unknown> }
export interface PageResult<T> { items:T[]; nextCursor?:string; hasMore:boolean; coverage:Coverage }
export interface ProfileSource { label:string; url:string; updatedAt:string|null }
export interface ModelProfile { id:string; name:string; platform:string; description:string|null; useCases:string[]; contextLimit:number|null; maxOutput:number|null; inputModalities:string[]; outputModalities:string[]; reasoning:Capability; toolCalling:Capability; structuredOutput:Capability; sources:ProfileSource[]; updatedAt:string|null; version:number; pricing:Array<{label:string;value:string;condition?:string}>; metrics:{tpm:MetricValue;rpm:MetricValue;ttft:MetricValue;tpot:MetricValue} }
export interface ModelTrendPoint { at:string; complete:boolean; values:Array<{model:string; performance:{tpm:number|null;rpm:number|null;ttft:number|null;tpot:number|null}}> }
export interface ModelComparison { models:ModelProfile[]; trends:ModelTrendPoint[] }
export interface DepartmentAnalytics { members:number;activeMembers:number;totalTokens:number|null;averageDailyTokens:number;perMemberDailyTokens:number|null;requests:number;outputTokens:number|null;outputShare:number|null;cacheHitRate:number|null;series:TimePoint[];models:ModelSlice[];departmentPareto:Array<{id:string;name:string;tokens:number;cumulativeShare:number}>;memberPareto:Array<{id:string;name:string;tokens:number;cumulativeShare:number}>;topUsers:Array<{id:number;name:string;tokens:number}>;performance:{tpm:MetricValue;rpm:MetricValue;ttft:MetricValue;tpot:MetricValue};coverage:Coverage }
export interface GatewayAnalytics { totalCalls:number;failedCalls:number;successRate:number|null;modelDuration:MetricValue;gatewayDuration:MetricValue;series:TimePoint[];totalUsers:number;newUsers:number;frequency:{high:number;medium:number;low:number;unclassified:number;coverage:Coverage};userSeries:TimePoint[];funnel:Array<{key:string;label:string;count:number|null;share:number|null;status?:string}>;preferences:Array<{department:string;total:number;models:Array<{model:string;count:number;share:number|null}>}>;coverage:Coverage;retentionCoverage:Coverage;preferenceCoverage:Coverage }
export type CostPaymentMethod = "subscription" | "payg" | "other";
export interface CostContributor { id:number;name:string;email:string;status:string;departmentId:string;department:string;deleted?:boolean }
export interface CostAccount { id:number;name:string;platform:string;type:string;status:string;expiresAt:string|null;deletedAt:string|null;registered:boolean;inherited:boolean;configurationMonth?:string;contributor?:CostContributor;paymentMethod?:CostPaymentMethod;requestCount:number;tokens:TokenBreakdown;platformCost:number;actualCost:number|null;savings:number|null;notes:string;updatedBy?:CostContributor;updatedAt:string|null }
export interface CostSummary { contributorCount:number;accountCount:number;platformCost:number;actualCost:number;savings:number;completedAccounts:number;totalAccounts:number }
export interface CostTrendPoint { month:string;platformCost:number;actualCost:number;savings:number;completedAccounts:number;totalAccounts:number }
export interface CostContributionDepartment { id:string;name:string;contributorCount:number;accountCount:number;requestCount:number;tokens:number;platformCost:number;actualCost:number;savings:number;completedAccounts:number;totalAccounts:number }
export interface CostUsageDepartment { id:string;name:string;requestCount:number;tokens:number;platformCost:number }
export interface CostDepartmentFlow { contributionDepartmentId:string;contributionDepartment:string;usageDepartmentId:string;usageDepartment:string;platformCost:number }
export interface CostDimensions { contributors:CostContributor[];departments:DepartmentOption[];platforms:string[];statuses:string[];paymentMethods:Array<{id:CostPaymentMethod;name:string}> }
export interface CostAccountPage { items:CostAccount[];total:number;page:number;pageSize:number;pages:number }
export interface CostDashboard { month:string;inProgress:boolean;summary:CostSummary;trend:CostTrendPoint[];contributionDepartments:CostContributionDepartment[];usageDepartments:CostUsageDepartment[];flows:CostDepartmentFlow[];accounts:CostAccountPage;dimensions:CostDimensions;coverage:Coverage }
export interface CostFilters { month:string;department:string;platform:string;q:string;contributor:string;paymentMethod:string;status:string;completeness:string;registration:string;page:number;pageSize:number }
