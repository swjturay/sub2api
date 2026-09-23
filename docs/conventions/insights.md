# Insights

## Product boundary

The separately built React application is served at the original origin under /insights/. The original site entry is labelled AI基础设施看板. Sub2API continues to own user identity, session refresh, roles, billing and model-call authorization. Public model catalogue responses are explicit projections; they must not expose group/account configuration or other users' request detail. The confirmed v1 contract is docs/INSIGHTS_V1.md.

## Statistical facts

All request-derived analytics filter and bucket by statistical record generation time. Existing usage.created_at remains the historical source. New usage, one client-call final result and any related final error share one explicitly assigned value; storage queues and retries must not assign different values. This time is not a precise start or completion timestamp, and cannot be recovered by subtracting duration.

Usage-record request counts and final-client-call request counts are distinct populations. Internal attempts and WebSocket connections are not new model calls; WS model turns are. HTTP 200 and the presence of usage do not prove a successful stream. Unknown final states and incomplete collection remain explicit coverage states. First-use cannot be inferred from the earliest retained or first newly observed record without trustworthy coverage.

Use the platform timezone consistently for boundaries and aggregates. Department membership is the current single complete attribute value, not an upstream resource group or a historical snapshot. Missing user values and invalid bindings are distinct. Preserve per-user/per-model daily facts and first/retention milestones before details expire; global totals cannot reconstruct them. Rollups must preserve sums, valid sample counts, source identities and coverage, not averages of averages.

## Gateway and storage isolation

Additional analytics must not change forwarding, quota windows, billing, output identity or client response behavior. Do not retain prompts, response bodies, credentials or raw sensitive upstream errors. All model-call transports actually supported by this fork need outcome coverage or an explicit incomplete state; no happy-path-only success claims.

Daily quota remains authoritative billing state. No report SUM may replace it for visual consistency. Current members exclude deleted users but include disabled users; all-gateway quality retains its separately defined anonymous/deleted-user history.

## Verification and presentation

Use real interfaces and database fixtures for formula, scope, time, lifecycle and retention tests, plus browser tests for login and independent filters. Synthetic fixtures belong only to tests/development and must never be production fallback data. No live paid model invocation is needed for tests. The model plaza is a version-controlled allowlist of formal production model names with profiles derived only from vendor documentation. Account mappings, request history, internal test names and experiments do not add catalog entries; administrators cannot edit model profiles, and there is no automatic discovery or periodic verification mechanism. Absent prices stay unknown rather than false/zero. No export, arbitrary SQL, drag-layout builder or member impersonation is added.
