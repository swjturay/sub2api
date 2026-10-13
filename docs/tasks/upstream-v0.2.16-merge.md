# Upstream v0.2.16 merge into cce-deploy

Date: 2026-10-13 (Asia/Shanghai). Scope: source integration and verification.

## Source

- Existing deployment branch: `f5e50c18e21da5ac53c2ccfbea62d495cfc7a8c6`.
- Official [v0.2.16 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.16):
  `01842576b3d5c1e2545b5a99ad555d36e28ed306`.
- Common upstream ancestor: `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`.
- 79 incoming commits, including merges, since the preceding intake.

The release adds Gemini explicit cache management/storage billing and Vertex
API-key/model discovery support, Claude Haiku 5.5, translated group names and
descriptions, and account/model-scoped temporary scheduling rules. It also
updates subscription pricing/context defaults and fixes protocol conversion,
image tool results, quota queries and frontend races/timers.

The release tag's VERSION file still contains 0.2.15. A separate follow-up
commit sets the runtime version to 0.2.16. Push the merge first and the version
correction second: the upstream range contains a `[skip ci]` housekeeping
commit, so the separate backend change provides a clean CI-triggering push.
See [GitHub's skip-workflow documentation](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/skip-workflow-runs).

## Integration decisions

Only `frontend/src/components/account/AccountUsageCell.vue` conflicted.
Retain the shared OpenAI/CPR usage predicate and the new Gemini third-party
API-key/Vertex predicates together. Existing CPR refresh/error regressions and
the new Gemini display regressions both remain enabled.

Regenerating Wire produces byte-identical output, preserving Insights
construction and wiring the new Gemini cache service into both the gateway
and graceful shutdown. No Go dependency changes are needed for this merge.

Gemini inference referencing a cache retains the existing explicit statistical
time and Insights terminal hooks. Cache create/list/get/renew/delete operations
remain control endpoints rather than inference calls; a middleware regression
test verifies that boundary and keeps ordinary Gemini generation observable.
Cache billing uses the mandatory usage-task path, preserving the fork's
in-flight reservation handoff and usage-failure reporting.

Insights UI/backend source and access rules, CCE release tooling/workflows,
and local setup scripts are unchanged. The new Insights boundary test extends
coverage without changing their production behavior.

## Database changes

All 298 existing SQL migrations remain byte-identical. Four new files bring
the total to 302; numeric prefixes shared with older fork migrations do not
collide because the runner tracks complete filenames:

- `242_group_i18n.sql`: additive group translation JSON, defaulting to empty.
- `243_gemini_cached_contents.sql`: gateway-owned cache/account bindings.
- `244_gemini_cached_contents_channel_usage.sql`: stable channel billing data.
- `245_gemini_cached_contents_mutation_lease.sql`: cache mutation leases.

Local PGlite execution applies all four twice and verifies existing group/cache
rows, defaults, unique cache IDs, translation/channel/lease fields and indexes.
This is an isolated schema check, not a production database operation.

Application rollback does not reverse these migrations. Existing group text
remains the fallback when translations are absent. After a future deployment
creates explicit upstream caches, rolling back to an older image removes the
new management/recovery code while upstream caches may remain until expiration;
review active cache resources before such a rollback.

## Validation

Local tools: Go 1.27.2, golangci-lint 2.14.0, Node 22.21.1 and pnpm 10.
The frontend install uses the frozen lockfile.

- Frontend lint and typecheck passed; critical suite: 39 files / 685 tests.
- Full frontend suite: 377 files / 2,938 tests. Production build passed.
- Insights typecheck, lint, 27 files / 109 tests and production build passed.
- Full backend unit and integration-tag suites and `go build ./...` passed;
  Go lint reported zero issues. Docker/database-dependent integration cases
  skip locally and still require the CI checks below.
- The new Insights/cache control-boundary regression passed.
- Wire regeneration exactly matches the merged generated file.
- Backend build with embedded frontend assets passed for version 0.2.16.
- Local setup: 20 tests; CCE release helper: 3 tests; shell syntax passed.
- Existing migration byte checks and new migration execution passed.

Docker-dependent database tests require Linux CI; the local host has no Docker
executable or daemon. Check CI, Insights CI and Security Scan on the final
version-correction commit before creating release images.

This task updates source only. It does not run the ACR image release workflow
or change production resources. Follow `deploy/cce/README.md` for a later release.
