# Insights

## Product boundary

The separately built React application is served at the original origin under /insights/. The original site entry is labelled AI基础设施看板. Sub2API continues to own user identity, session refresh, roles, billing and model-call authorization. Public model catalogue responses are explicit projections; they must not expose group/account configuration or other users' request detail. The confirmed v1 contract is docs/INSIGHTS_V1.md.

## Statistical facts

All request-derived analytics filter and bucket by statistical record generation time. Existing usage.created_at remains the historical source. New usage, one client-call final result and any related final error share one explicitly assigned value; storage queues and retries must not assign different values. This time is not a precise start or completion timestamp, and cannot be recovered by subtracting duration.

Usage-record request counts and final-client-call request counts are distinct populations. Internal attempts and WebSocket connections are not new model calls; WS model turns are. HTTP 200 and the presence of usage do not prove a successful stream. Unknown final states and incomplete collection remain explicit coverage states. First-use cannot be inferred from the earliest retained or first newly observed record without trustworthy coverage.

Use the platform timezone consistently for boundaries and aggregates. Department membership is the current single complete attribute value, not an upstream resource group or a historical snapshot. Missing user values and invalid bindings are distinct. Preserve per-user/per-model daily facts and first/retention milestones before details expire; global totals cannot reconstruct them. Rollups must preserve sums, valid sample counts, source identities and coverage, not averages of averages.

## Department consumption trends

Department analytics buckets expose both department and provider-qualified model Token breakdowns. Build them from the same filtered aggregates as bucket totals and merge them by identity across the daily/detail retention boundary. Never reconstruct time-bucket categories from whole-window proportions. Missing breakdowns remain unavailable, while an absent category in a supplied breakdown means zero recorded usage.

## Gateway and storage isolation

Lifecycle rebuilds share call, historical usage and nonempty per-user daily
activity evidence for observed activity and coverage-gated, launch-scoped first
anchors and returns. Prefer exact evidence over a same-day rollup timestamp;
retain stored milestones after detail cleanup. A newer instrumentation start
must not reset existing users' first-use dates. Mixed cohorts may contain both
confirmed returns and users whose observation window has not matured.

Additional analytics must not change forwarding, quota windows, billing, output identity or client response behavior. Do not retain prompts, response bodies, credentials or raw sensitive upstream errors. All model-call transports actually supported by this fork need outcome coverage or an explicit incomplete state; no happy-path-only success claims.

Daily quota remains authoritative billing state. No report SUM may replace it for visual consistency. Current members exclude deleted users but include disabled users; all-gateway quality retains its separately defined anonymous/deleted-user history.

## Cost completeness

Summary and contribution-department completeness count only registered accounts
eligible for the selected month's account table, before table-only search,
status filters and pagination. Exclude zero-request accounts in every month;
the existing current-month zero-token exclusion still applies first. Count an
explicit zero spend as complete, never a missing value. Exclude unused accounts
from both numerator and denominator without changing saved historical amounts
or historical contribution-account counts. Cost trends show only estimated
price, actual spend and savings, without a completeness series or percent axis.

## Verification and presentation

Use real interfaces and database fixtures for formula, scope, time, lifecycle and retention tests, plus browser tests for login and independent filters. Synthetic fixtures belong only to tests/development and must never be production fallback data. No live paid model invocation is needed for tests. The model plaza is a version-controlled allowlist of formal production model names with profiles derived only from vendor documentation. Account mappings, request history, internal test names and experiments do not add catalog entries; administrators cannot edit model profiles, and there is no automatic discovery or periodic verification mechanism. Absent prices stay unknown rather than false/zero. No export, arbitrary SQL, drag-layout builder or member impersonation is added.
