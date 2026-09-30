# Development Journal

## CPR quota refresh failures (2026-09-30)

Production account 40 lacked its CPR management key. The quota API swallowed
that configuration error, stamped the query time as fresh, and created zero
quota bars while attaching local accounting statistics. Preserve actual quota
freshness and missing windows, return sanitized admin error codes/messages,
and display the warning alongside cached values and the refresh button. A
failed refresh must not clear an account's recoverable error. The missing key
was repaired through the admin API after validating the exact bound account;
the account remains manually paused. Both displayed windows now match CPR.

The real failure reproduced before the patch, as did the backend regression
and two UI tests. Targeted CPR/account-usage tests and all 56 account usage
component tests pass; frontend production build and changed-file lint pass.
No billing, generation transport, retry policy or database migration changes.

Separate log diagnosis found 416 CPR routing rejections with `not_sent` from
14:18 to 14:58 CST. One converted request retried 230 times in 120 seconds
despite logging a limit of 3: CPR inherits the direct OAuth transient-429
deadline, which intentionally overrides the pool count and delays cooldown.
CPR quota failures are masked as generic capacity errors on this path. This
is inappropriate for an exhausted one-to-one binding. Recommended follow-up:
apply the configured retry count to CPR, switch bindings promptly and use
confirmed quota/reset information for cooldown. Keep direct OAuth policy and
no-replay-after-output/cancellation rules. Retry policy is unchanged here.

## CPR publication and database-test isolation (2026-09-30)

Published the native-relay implementation as `fb61caa11` on `cce-deploy`.
Deployment assessment found baseline CI `36652265720` failed two dashboard
integration assertions because the observation round-trip test retained two
records in the shared database. Keep its real batch-insert path and assertions,
but clean up its usage and owned fixtures at test exit. Include CPR completeness
in the real PostgreSQL JSONB round trip. Also remove an unused donor raw-switch
constant reported by exact-source lint; the API-key exclusion test keeps the
literal legacy key. No runtime behavior changes. Production deployment is still
pending exact-source CI, the release image,
additive migration preflight, and real CPR cancellation/accounting verification.

## CPR native relay and observed usage (2026-09-30)

Targeted intake from KLNO d75d6b87 onto CCE 6335c8a2, following the confirmed
[task scope](tasks/cpr-integration.md) and [cancellation ADR](adr/0002-cpr-cancellation-and-observed-usage.md). CPR native Responses now uses dedicated HTTP/SSE and WS relays; Chat/Messages
retain conversion, mappings and full history. Both CPR count endpoints estimate
locally. No CPR kernel changes, production switch or API-key raw switch.

Received usage survives downstream write failures, nil-result error paths and
client cancellation. Complete/partial/unknown evidence is stored in existing
admin-only observation JSONB. Independent attempt billing keys prevent failover
dedup collisions; Cyber uses one captured bill and preserves its risk event.
User/account execution leases stop releasing before CPR teardown; normal WS
close is cancelled, not an upstream error. Per-turn image policy, Insights,
passive provenance and existing mapping remain active. CPR inference headers
do not update quota snapshots or fabricate public route observations.

Validation: final `go test -tags=unit -p 2 ./...` and `go build -p 2 ./...` passed;
117 targeted top-level backend tests and 36 frontend table/i18n tests passed.
Frontend production build, final typecheck, changed-file ESLint, root repository
validation and diff checks passed. Local/remote state and live-validation limits
are recorded in the task report. The user subsequently authorized publication
to `cce-deploy` and a deployment-readiness assessment; production is unchanged.


## OAuth local token counting (2026-09-30)

OAuth and OpenAI setup-token accounts now estimate both Responses input_tokens
and Anthropic count_tokens locally before credential retrieval or refresh.
Model mapping, Anthropic conversion, tiktoken estimation and minimum fallback
are reused. Logs mark direct estimates with reason=oauth_local and upstream
status zero. API-key policy and CPR's distinct native/Anthropic paths stay as-is.

Production read-only evidence motivated this optimization: the retained ingress
window (2026-09-29 19:24 through 2026-09-30 08:37 +08:00) contained 28 Claude Code
count requests and no Responses input_tokens requests. In the shorter retained
application window, 11 OAuth requests received upstream 401 before local fallback
(0.55–5.78 seconds); 15 CPR requests returned upstream 405. This patch avoids the
OAuth round trip; it does not address CPR 405 or alter inference usage/billing.

Regression coverage exercises both endpoints for OAuth/setup-token accounts
without credentials, malformed input, zero transport calls and no account error
or temporary-unschedulable mutations; API-key and CPR transport boundaries remain
covered. Changes are local and have not been deployed.

Validation: full service and handler suites passed with
`go test -p 2 -tags=unit ./internal/service ./internal/handler -count=1`
(206.488s / 39.404s); `go build -p 2 ./...`, the operations repository validator
and diff whitespace checks passed. Local HEADs match remote main/cce-deploy;
existing unrelated working-tree changes were preserved. No commit or push.

## Codex protocol fidelity and passive observations (2026-09-29)

Selected intake from KLNO d75d6b8 onto CCE 6f23460, preserving the earlier proxy,
OAuth exchange-cookie isolation and Lite/compression changes:

- Preserve native `call_*` IDs and align existing turn metadata model values
  after mapping, in HTTP normal/passthrough bodies/headers and WS final frames.
  Preserve CCE ASCII handling, unknown metadata and reasoning selection.
- Relay safety hints through Responses and Chat/Anthropic bridges; persist
  presence separately from parsed enabled values. Native WS coverage remains
  explicitly unknown; WS-to-HTTP carries actual HTTP observations.
- Replace donor raw-cookie/shared-jar route logging with immutable per-attempt
  account-scoped HMAC digests. Separate sent and received values; only a bounded
  unverified gateway hint is decoded. No Cookie replay or routing-policy change.
- Add nullable JSONB migration 243 and adapt single/batch inserts, scan and
  admin-only DTO. Admin usage has an optional expandable observation column,
  including migration of existing column preferences.
- Review caught and fixed bridge observation/header omissions and Lite's second
  pass reverting finalized metadata. Regression tests cover these compositions.

OAuth local token estimation is intentionally unchanged. It is a count-endpoint
compatibility optimization, not inference acceleration or billing accuracy.
The research report in the operations repository records the value assessment.

Validation: `go test -p 2 -tags=unit ./...` passed; after review fixes, the full
service package passed again (198 seconds), followed by `go build -p 2 ./...`.
SQL mock roundtrip covers NULL/HTTP/WS observations, single insert shape and
batch argument wiring; admin-only JSON exposure and concurrent snapshot
regressions passed. Frontend table/view/i18n suites passed (49 tests), as did
typecheck, changed-file ESLint and production build. Existing bundle-size and
Browserslist-age warnings remain. The root operations validator and whitespace
checks passed. Standards review has no remaining blocker; Spec review's three
findings were fixed and re-reviewed successfully.

The real PostgreSQL integration test was compiled but explicitly skipped by
the harness because Docker remains unavailable, even after attempting to start
Docker Desktop. Migration 243 and database roundtrips still require execution
before release; local Go lint is unavailable. No production deployment,
commit, push, Cookie replay or Guardian injection is included.

## Selective KLNO reliability intake (2026-09-29)

Compared CCE `6f23460cff9ef2aff144a11654457b842c01c286` with KLNO
`d75d6b870697e6380f9f81765b73c0f36a9ea4f9`. The user selected three changes:

- Refresh-token import fails before network access on proxy lookup errors or
  missing proxy objects.
- OpenAI token exchange/refresh disables cookie storage and replay while keeping
  pooled connections. Cookie policy separates cache entries. Browser and Codex
  management-plane factories retain their existing behavior.
- Lite payload edits retain valid zstd encoding, length and replay bytes. Native
  Lite requests omit synthesized instructions only within the existing device
  profile and unchanged-model boundary. All three compact fallback paths restore
  missing instructions; explicit client instructions remain authoritative.

Adapted selected code and Lite regressions from KLNO, without taking its raw
relay, turn-state, cookie replay, Guardian, token-estimation or database changes.
Added proxy nil/error tests, a two-account cookie replay test, and real HTTP
captures for compressed builders and native OAuth/setup-token Lite requests.

Validation: targeted regressions passed, followed by `go test -tags=unit ./...`
and `go build ./...`. Additional HTTP capture tests for native OAuth/setup-token
Lite requests passed after review. The operations repository validator and
whitespace checks passed. Local Go lint is unavailable; container-backed
integration and exact-source CI remain release gates. No commit, push, image
publication or production deployment is part of this intake.

## Insights release validation correction (2026-09-29)

The release CI caught unchecked type assertions in the department bucket regression helper. Added explicit response/bucket type checks with test failures and helper attribution, preserving the existing regression coverage and production behavior. The release remains gated on all exact-source CI checks; no lint rules were weakened.

## Model plaza annotation refinements (2026-09-29)

- Replaced routing-platform presentation with manufacturer classification across filters, counts, cards, lists, details and comparison. Gemini contains only Gemini families, Claude appears under Anthropic, and OpenCode's DeepSeek entries appear under DeepSeek. Backend identities, distinct routing entries and their measured values are preserved; no aliases or performance averages are merged. Gemini uses the existing Gemini star mark in blue.
- Expanded capability filtering from three to nine options using declared input/output modalities and an explicit 128K-token context threshold. Removed the requested sidebar explanation, price-count summary and global performance note.
- Fixed catalog/comparison performance to 24 hours. Moved the independent 24-hour/7-day control into model details, with fresh default selection on reopening, request cancellation, no stale values under a new period and in-place error retry. Detail subtab changes preserve its selected window.

Validation: Insights lint, 26 test files / 104 tests and TypeScript/production build passed. Regressions cover manufacturer separation, inferred capability boundaries, independent detail windows, late responses, retry and closing pending requests. Browser preview confirms Gemini-only results, combined image-generation filtering and the moved detail control. The local preview retains clearly labeled example performance/prices. No commit, push or production deployment in this refinement.

## Model catalog discovery and presentation (2026-09-29)

- Reorganized the Insights model plaza around a search header, platform/capability/input filters, four sort modes, card/list views and 12-item pagination. Used the [New API pricing catalog](https://github.com/QuantumNous/new-api/tree/f116414284162ad15d8925f7bca494c109b83e93/web/src/features/pricing) as an information-hierarchy reference; no New API code, artwork or dependencies were copied. Platform marks reuse the existing repository frontend artwork.
- Cards and rows expose both display names and invocation identifiers, context/capabilities, reference-price summaries and aggregated performance. Unknown capability metadata does not satisfy a supported filter; absent measurements sort last, genuine zero remains zero, and conditional prices point to full details instead of appearing unconditional.
- Kept 2–4-model selection across filtering, paging and view changes. Model details now use a right-side dialog with specification/performance and pricing/source views, retaining full tier conditions, safe source links and focus restoration. Fixed inherited dialog translation after browser inspection and disabled sticky filtering on short desktop windows.
- Updated the design and Insights conventions. API contracts, catalog contents, authentication, aggregation and production configuration are unchanged.

Validation: Insights lint, 25 test files / 99 tests, TypeScript and production build passed. Browser checks covered platform filtering, card/list switching, latency sorting, comparison, details, price/source access and focus restoration. DOM checks found no horizontal document overflow at 1366, 1440 and 1920 desktop widths; inspected light/dark screenshots, the list and the corrected detail drawer. Local preview at `http://127.0.0.1:4189/insights/models` uses the repository's 89-model catalog with explicitly labeled example prices/performance. Preview fixtures are ignored and cannot enter the production bundle. No live paid inference or deployment was performed. The existing large-bundle build warning remains.

## CCE upstream v0.2.9 intake (2026-09-29)

Base: `5019d448a90043b5c358482c0b35b0916106beb7`. Upstream: `v0.2.9` (`4c00df2e0183e2c70b7fa8ba45914205e36aad0c`). Target: `cce-deploy`.

- Merged the upstream release, including wildcard model allowlists, Responses Beta forwarding and context-window rollover, client-disconnect classification, quota reset/backoff, billing corrections, protocol conversion fixes, model-plaza video pricing and CC Switch import fixes.
- Resolved the client-configuration conflicts by retaining CCE's default config without a mandatory `model_catalog_json` file. Preserve the optional catalog download and Windows save-path display; verify that the generated TOML contains no `%userprofile%` path. Existing local client setup, authentication modes and shared-AI providers remain intact.
- Updated the CCE wire-entry regression to expect independent Beta tokens on both forwarding paths. Default-instruction synthesis still proves that the two paths are distinct before their cache keys are compared.
- Preserved CPR transport/identity boundaries, platform quotas, gateway observations, Insights costs/retention and CCE release tooling. No database migration or production configuration changed. Set `VERSION` to `0.2.9`; the upstream release tag still contains `0.2.8` in that file.

Validation: the full backend unit run passed except for the obsolete Beta assertion and three missing-`sh` environment failures; after correcting the assertion and adding Git's `bin` to PATH, both complete failing packages passed. `go build ./...` passed. Frontend lint, typecheck, all 338 files / 2,556 tests and production build passed; the updated 37-test client-configuration suite passed separately. Insights lint, 90 tests and TypeScript/production build passed. CCE release-helper tests and Compose security, gateway environment and runtime-resource checks passed. Whitespace checks passed. Docker Desktop is unavailable locally, so container-backed integration and Go lint remain for the exact-commit CI after pushing. This change synchronizes source only; no image release or production deployment was performed.

## Cost completeness and monetary trend (2026-09-28)

- Matched summary and contribution-department completeness to monthly table usage eligibility. Zero-request accounts no longer enter either numerator or denominator, including historical months; the current-month zero-token rule is unchanged. Preserve explicit zero versus missing spend, stored amounts, historical cost totals and contribution-account counts.
- Removed the completeness series, legend entry and secondary percentage axis from the cost trend. Keep estimated price, actual spend and savings, including gaps for missing entries and confirmed-zero values.
- Added dashboard regressions for historical/current months, pagination, table-only search, all-unused months, paid/unpaid unused accounts and department completeness. Added real PostgreSQL and rendered-page checks.

Validation: backend completeness tests and the chart-series assertion failed before the fix and passed after it. Insights lint, 90 tests and TypeScript/production build passed, as did focused backend Insights/handler/routes tests and the isolated PostgreSQL 18 cost suite. The temporary database container was removed; no production data was changed. Browser verification confirms the monetary-only trend. The local page still proxies production, so new completeness counts require backend deployment. Deployment is separate from this code change.

## Gateway duration units and timing diagnosis (2026-09-28)

- Display both gateway quality durations in seconds, converting the millisecond API values only at the page boundary. Preserve null, zero and small positive values; other metrics and API units are unchanged. Rename the gateway metric to pre-forward time and explain both timing boundaries in tooltips.
- Read-only production audit of the selected September 22-28 window found mean pre-forward time about 1.689s, median 0.201s, P95 7.517s and P99 32.309s. The 5,955 samples at or above 10s (about 4.2%) contributed 69.1% of the total elapsed time. Matching usage identifies DeepSeek Flash in 4,463 of those tail samples. September 27's mean was 3.929s versus September 28's approximately 1.115s.
- The collector measures from middleware entry to the first transport send, including preparation/authorization/scheduling/concurrency waits. It is not pure CPU time or model response time. Persisted successful-request evidence does not break this interval down by phase, so queueing is not asserted as the sole cause. No production data, concurrency settings, forwarding code or instrumentation was changed. Private aggregate audit output is retained in `/root/insights-followup-20260928/timing-audit-*.json`.

Validation: page regression reproduced `1.9万ms` before the change; Insights lint, 89 tests and TypeScript/production build passed. Local browser shows approximately 18.7s and 1.7s. This turn's frontend changes remain local and undeployed.

## Gateway repair tail and historical retention backfill (2026-09-28)

- Found and repaired 129 missing-hook Chat outcomes between the original repair cutoff and old backend retirement, after an explicit-failure check and rollback rehearsal. Existing failures, usage, tokens and costs were unchanged.
- Lifecycle rebuild now reuses historical usage and nonempty daily activity for trusted, launch-scoped anchors and returns. Prefer exact timestamps over same-day rollups and retain milestones after cleanup; coverage gates and batch isolation are unchanged.
- Backfilled existing history under the lifecycle lock with before/after backups. The 119-user funnel now has 105 first users, 99 next-day returns, 92 seven-day returns and 68 thirty-day returns. One/23 users are still younger than seven/thirty days. Local charts show confirmed positive counts and explain immature users in the tooltip.
- Private evidence and guarded rollback scripts are in `/root/insights-followup-20260928`; see [the follow-up audit](INSIGHTS_GATEWAY_AUDIT_2026-09-28.md). At 15:08 the live September 28 success rate was 90.31%; most remaining failures were recorded as request rejections, not the repaired Chat signature.

Validation: the new PostgreSQL regression failed before the fix; all Insights tests including real PostgreSQL integration passed after it. Focused backend unit tests and full build, Insights lint, 85 frontend tests, TypeScript/build, production API/history reconciliation and health check passed. The isolated test container was removed. Local browser shows the restored funnel. Production data backfills are complete without a service restart; this turn's code and tooltip changes remain local, uncommitted and undeployed.

## Historical gateway outcome repair (2026-09-28)

- Applied the explicitly approved historical policy to missing-hook Chat Completions candidates before `2026-09-28 13:35:27.598476+08:00`: preserve explicit failures and classify remaining candidates as success. This assumption does not constitute reconstructed terminal evidence.
- Rehearsed with rollback, then atomically repaired 45,873 call outcomes, removed their stale error facts and rebuilt call-only daily aggregates. Five candidates with explicit failure evidence and all other failure classes were retained. Usage, tokens, billing and costs were unchanged.
- Production API confirms September 25 success rate 96.65% (previously 44.50%) and September 26 93.85% (previously 31.05%). Raw/daily counts reconcile and no target stale errors remain.
- Backup schema `insights_repair_20260928`, private ops dump and guarded rollback are retained. See [the audit and executed repair record](INSIGHTS_GATEWAY_AUDIT_2026-09-28.md).

Validation: post-commit database assertions, authenticated production API reconciliation and public health check passed. No restart, commit, push or deployment. New handler hooks remain local; the old production code can still misclassify requests after the fixed cutoff until deployment.

## Gateway measurements and presentation (2026-09-28)

- Read-only production diagnosis found missing Chat Completions identity/success hooks. September 25/26 have 8,058/19,456 interrupted records with default transport, missing model/provider and matching Chat usage; most are DeepSeek Flash streams. Added explicit successful-terminal recording and the shared usage timestamp; errors with partial usage are not upgraded to success. Historical outcomes were not rewritten.
- Corrected retention coverage: trusted users without observed calls are inactive, not missing history. Genuine missing first/return coverage remains visible, and immature milestones remain pending.
- Department preferences recover legacy composite providers from unambiguous, identity-scoped usage records without changing counts. Daily-only legacy identities stay unknown. Catalog-aware frontend normalization merges equivalent series without guessing ambiguous providers.
- Removed the unclassified-user card and moved gateway/panel prose to accessible tooltips. Kept missing-collection warnings separate from frequency categories.
- See [the read-only audit](INSIGHTS_GATEWAY_AUDIT_2026-09-28.md) for counts, evidence limits and historical-repair requirements.

Validation: frontend lint, 84 tests and build; focused backend unit tests and full build; real PostgreSQL integration including the previously pending zero-request cost-table tests passed. The isolated test container was removed. Browser checks passed against the local frontend; its API still targets production, so backend changes are not live. No commit, push, deployment or production data change.

## Hide unused account-month table rows (2026-09-28)

- Historical account tables previously retained zero-request accounts because only the current month's zero-token rule excluded unused rows. Apply a request-count check to the table in every month, before table filters and pagination.
- Keep persisted monthly records and historical summary/trend/department costs unchanged. Historical requests with zero tokens remain visible; the existing current-month zero-token accounting rule remains in effect.
- Added dashboard regressions for historical pagination, all-unused empty results, searching an unused account, current-month behavior and unchanged historical spend. Updated PostgreSQL fixtures to keep ownership/stop scenarios backed by actual requests.

Validation: the new dashboard regression failed on the original code and passed after the fix. Focused Insights, migration, handler and route unit tests and whitespace checks passed. PostgreSQL integration was explicitly attempted but skipped because `INSIGHTS_TEST_DATABASE_URL` is unset. No production data or deployment changes.

## Cost test lint gate (2026-09-28)

- Exact-source PostgreSQL integration tests passed for the mixed-version contributor trigger. The CI lint gate also checked newer cost regressions and flagged unchecked test cleanup and type assertions.
- Matched the existing explicit cleanup pattern and added checked dashboard assertions. Focused Insights and migration tests passed locally; publication remains gated on a fully green final-source CI run.

## Mixed-version cost contributor writes (2026-09-28)

- Before deploying migration 242, added a monthly-write trigger that synchronizes account-wide ownership in the same database statement. Both the old backend and the new backend now update the same account record during a gradual rollout or application rollback. Stop events do not erase ownership.
- Kept the application write as a single monthly upsert so both backend generations take locks in the same order. Added integration checks for a legacy writer, transaction rollback, and replaying the initial backfill without overwriting current ownership.
- Migration 242 has not yet been applied to production. Release deployment is gated on the exact final commit's PostgreSQL, application, frontend and security CI.

## Searchable cost contributor selection (2026-09-28)

- Replaced the cost editor's contributor Select with a single-choice Command/Popover using existing design tokens. Search matches Chinese names, email addresses and departments without case sensitivity; candidates show department and email to distinguish duplicate names and use user IDs as their identity.
- Added empty results, existing-selection display, disabled-state preservation, automatic search focus, keyboard navigation, close-on-select, Escape cancellation and search reset on reopening. Filtering stays local and does not mutate saved account configuration.
- Aligned Radix Dialog and Select with the existing Popover focus/dismissable-layer versions. The older Dialog had a separate focus-scope stack that immediately dismissed the nested search popover; nested modal mode instead caused conflicting focus traps. The matching primitives support the non-modal search layer without additional scroll locking.
- Added eight component regressions inside the actual editor Dialog, including duplicate names, empty/current initial selections, missing current users, no results, focus restoration and the adjacent payment Select.

Validation: frozen-lockfile install, Insights lint, 19 files / 81 tests and TypeScript/production build passed. Restarted local Vite with dependency re-optimization. Browser checks verified Chinese-name filtering, duplicate-name email display, keyboard selection, trigger summary and the payment dropdown; all test edits were canceled without saving. The local preview remains at `http://127.0.0.1:4178/insights/costs` with the production API proxy. No production data change or deployment.

## Account-wide cost contributors (2026-09-28)

- Added migration 242 for one contributor per logical account, shared across all accounting months. Backfill selects the latest edited registered monthly record (month breaks timestamp ties), ignores stop records, and preserves every monthly amount. Legacy monthly contributor snapshots remain for compatibility; dashboard attribution now reads the account-level record.
- Historical months before the first registration use the earliest registered configuration for registration/payment defaults, without copying actual spend. Existing stop events take precedence over future registration. Stopping an account never clears its shared contributor.
- Account tables, contributor/department filters, contribution summaries, usage flows and trend attribution use the same account-wide contributor. Saving the shared contributor and the selected month's record is atomic. The editor labels the field as account-scoped and explains the historical impact in its tooltip.
- Added transaction commit/rollback regressions and migration contract tests. Expanded the PostgreSQL integration fixture to execute the actual migrations and cover historical defaults, changing ownership from an earlier month, independent spend, department/trend/flow attribution, failed-save rollback, stop/restart boundaries and idempotent backfill conflict resolution.

Validation: `go test -tags=unit ./internal/insights ./migrations ./internal/handler ./internal/server/routes -count=1`, `go build ./...`, Insights lint, 73 frontend tests, TypeScript/production build and `git diff --check` passed. PostgreSQL integration was skipped because `INSIGHTS_TEST_DATABASE_URL` is unset and Docker Desktop is unavailable. No production data or deployment changes; local preview still uses the production backend, so account-wide attribution requires the new backend and migration 242.

## Cost month picker and department coverage (2026-09-28)

- Replaced the native cost month input with the shared analytics Button/Popover styling, calendar icon, bounded year navigation and a twelve-month grid. Selection closes the popover; Escape cancels without changing the filter. Existing statistics start/current-month limits are retained.
- Fixed the cost endpoint leaking the department binding status `configured` into coverage metadata. Healthy bindings now return `complete`; missing/invalid bindings retain their status and diagnostic detail. The frontend narrowly accepts the legacy `department_attribute/configured` pair so the local preview works with the deployed backend without hiding genuine warnings.
- Added regression coverage for backend binding states, frontend healthy/invalid/missing/partial coverage, month bounds, year navigation, selection and Escape/focus behavior. Both healthy-coverage regression tests failed before the fix.

Validation: Insights lint, 18 files / 73 tests, TypeScript/production build, backend Insights tests and `git diff --check` passed. Browser verification covered month changes and restoration, keyboard interaction, removal of the false warning and unchanged toolbar/scroll geometry on opening. Checked the 1024px desktop layout and the month popover at 390px; the existing application-wide 1024px minimum width remains unchanged. Local Vite uses the production API; only read-only dashboard requests were made. No production deployment or data changes.

## Hide idle accounts from current-month costs (2026-09-28)

- Exclude logical accounts with zero total monthly tokens from the ongoing month's account table, summary, completeness, contribution departments, usage departments, flow matrix, and trend point. Count input, output, cache-write and cache-read tokens after rolling child accounts into their parent. Use one platform-timezone month value per dashboard request.
- Retain saved contribution settings and actual spend. Accounts become visible again when usage arrives; historical months keep all accounts and costs. Department usage queries use the same included root account IDs as the summary.
- Added query regressions for zero-token paid accounts, cache-only activity, historical cost retention and restricted department aggregation. Extended the PostgreSQL fixture for child-only activity, a timezone month boundary and automatic reappearance without re-entering cost.

Validation: Insights, handler and routes package tests, `go build ./...`, and `git diff --check` passed. PostgreSQL integration execution is unavailable because Docker Desktop is not running and `INSIGHTS_TEST_DATABASE_URL` is unset. The local Vite preview still proxies the production backend, so this backend change requires a backend rollout to appear there.

## Subscription requests share user platform quotas (2026-09-27)

Base: `b359b2b30883b839968f55b8f7180b05c383b493`. Branch: `cce-deploy`.

- Removed the subscription exemption from user platform quota admission and from both committed and legacy usage accounting. Subscription requests still consume their subscription allowance and do not deduct the user's balance; the same `ActualCost` also counts toward the user's daily, weekly and monthly platform quotas.
- Reused the existing quota records, Redis counters, flusher/direct persistence and administrator controls. Quotas are shared across a user's keys, groups and billing modes; Composite requests use their resolved provider. No migration, new switch or model-level quota was introduced. Existing usage is preserved; historical subscription usage is not backfilled.
- Updated the Chinese and English administrator subscription notice. Existing zero limits now block subscription requests too; unconfigured limits, free usage and simple-mode exemptions retain their existing semantics. Post-request accounting and its in-flight overshoot/failure behavior are unchanged.
- Added regressions for subscription/platform limit enforcement, daily window reset, Composite DeepSeek accounting, cross-key/group/mode sharing, provider/user isolation, actual-cost accounting without balance deduction, billing replay deduplication, both persistence modes and the legacy fallback.

Validation: full backend `make test-unit`, targeted quota regressions with `go test -race -tags=unit`, `golangci-lint run ./...` (0 issues), backend `make build`, frontend `make test-frontend` (lint, typecheck, 30 files / 423 tests) and frontend production build including all locale compilation checks passed. Local backend validation used Go 1.27.1 and golangci-lint 2.13.0; frontend dependencies used the frozen lockfile with pnpm 9.15.9. `git diff --check` passed; the local base and remote `origin/cce-deploy` both remained at the revision above. PostgreSQL/Docker integration tests and live production validation were not run. No production deployment or production data change was performed.

## Insights cost data dashboard (2026-09-24)

- Added the administrator-only `成本数据` module for internal shared-account cost management. Monthly account events inherit contributor and payment method until an administrator stops the contribution; exact-month actual USD spend remains nullable so missing input is distinct from confirmed `0.00`.
- Reused the existing account statistical cost formula and rolls linked Spark child usage into the logical root account. The dashboard exposes monthly completeness, a 12-month cost/savings trend, contribution and usage department summaries, and a contribution-to-usage department cost-flow matrix. All historical department attribution follows each user's current department.
- Added direct monthly editing for existing Sub2API users, payment methods `订阅` / `即用即付` / `其他`, last-editor metadata, unconfigured-account discovery, bounded pagination, and account/contributor/payment/status/completeness filters. Credentials and user-side billed amounts are not queried or returned.
- Added migration, validation, aggregation and adapter regressions, including missing-versus-zero spend, negative savings, deleted-account status, and a PostgreSQL fixture for parent/child account rollup. Browser checks covered 1440x900 and 1024x768, edit inheritance and confirmed-zero preview, chart rendering, and document-level overflow.
- Follow-up UI review aligned the monthly cost filters with the shared compact analytics controls, replaced the two top-level Radix Select menus with stable non-modal popovers, and renamed the user-facing calculated amount to `预估价格` with an explicit standard-price-times-usage tooltip. The loaded-page browser probe confirmed zero change in scroll position, toolbar position, main-content width, and horizontal offset while opening the platform menu.

Validation: full backend `go test -tags=unit ./...`, backend production build, Insights ESLint, TypeScript, 18 files / 66 tests and production build passed. The PostgreSQL integration fixture compiled but was skipped because `INSIGHTS_TEST_DATABASE_URL` was unset and the local Docker Desktop daemon was unavailable. No production deployment or production data change was performed.

## CCE 0.2.8 deploy branch intake (2026-09-24)

Base: `8996837eb845015c1f14f7632b414ac63ea7de19` (`cce-0.2.7`). Upstream: `v0.2.8`. Target: `cce-deploy`.

- Integrated the complete upstream 0.2.8 release while preserving the CCE CPR boundary. CPR remains an AI-traffic passthrough, while Sub2API continues to own quota enforcement, scheduling, model availability, and rate limiting; CPR quota state is read only through the CPR Admin API.
- Kept `CodexBackendClientFactory` and `PrivacyClientFactory` as separate transports. Codex/WHAM usage, quota reset, and spendable-credit traffic uses the Codex backend identity; privacy, account, subscription, and referral traffic uses the browser-oriented privacy identity without silent fallback between them.
- Combined the upstream Codex metadata serializer with CCE raw-preserving behavior: HTTP headers are ASCII escaped, while opaque WebSocket and request-body metadata preserves the original JSON text. Preserved the CCE shared-AI OpenCode providers and added the upstream GPT-6 variants and Claude Opus 5.5 catalog entries.
- Made `cce-deploy` the sole CCE deployment source. Manual release dispatch rejects other refs, and release commits must be reachable from the remote deployment branch. Also made the upstream release-matrix tooling deterministic under UTF-8 and portable to Windows test environments.
- Removed the unused whole-object Codex metadata marshal wrapper reported by the production CI lint gate; active header and opaque-body serialization paths remain unchanged.

Validation: full backend unit suite and build; frontend lint, typecheck, 423-test critical suite, targeted regressions, and production build; release Python compilation and 10 tests; workflow YAML parsing, `actionlint`, release shell syntax, and whitespace checks passed. The Compose simple-mode runtime test could not run because the local Docker Desktop daemon was unavailable.

## Insights fixed statistics start (2026-09-24)

- Removed the persisted and API-visible `trusted_since` boundary. User-facing statistics now use only the configured system launch date, exposed as `statistics_start_date`; all dates from launch through today are complete, and absent usage rows are zero-use days.
- Added `INSIGHTS_STATISTICS_START_DATE` / `insights.statistics_start_date`, fixed production CCE to `2026-06-01`, and made the release patch remove the obsolete trusted-history environment variable during rollout.
- Scoped first-request and retention anchors to the launch window, including accounts created before launch, and removed the heatmap's data-gap state. Internal collection health remains available for operations but cannot move the statistical range.

Validation: targeted and package-level backend tests, backend production build, Insights TypeScript/Lint/58 tests/production build, CCE release tests, Compose configuration, and whitespace checks passed. PostgreSQL integration tests compiled but were skipped because the local Docker daemon was unavailable; production data was not modified during validation.

## Personal Insights production launch boundary (2026-09-24)

- Confirmed **2026-06-01** as the Sub2API production launch date in the platform timezone. The CCE release command now sets both trusted Insights collection and `INSIGHTS_TRUSTED_USAGE_HISTORY_FROM=2026-06-01` on the backend Deployment.
- This boundary certifies the existing billing ledger rather than backfilling raw data. Existing Token and amount rows are aggregated as recorded; calendar dates on or after launch with no usage rows are zero-use days. Dates before launch are marked `统计范围外`, while explicit post-launch collection gaps remain `数据缺口`.
- Heatmap coverage evaluation starts at the launch boundary, so pre-launch dates no longer make an otherwise complete year appear partially collected. Operations documentation records the same rule and requires the repository-owned CCE release command to be refreshed before rollout.

## CPR Composite Codex image capability (2026-09-23)

- Fixed Codex model-manifest capability aggregation for Composite groups that mix OpenAI OAuth and CPR accounts. CPR accounts now use the same known GPT image-input fallback as OAuth when their model metadata omits modalities, so a schedulable CPR account no longer downgrades `gpt-5.6-sol` to text-only.
- Kept explicit upstream metadata authoritative: a CPR snapshot that declares only `text` still narrows the Composite group capability to text-only. The conservative all-candidates intersection remains unchanged.
- Added regression coverage for both the missing-metadata fallback and explicit text-only behavior.

Validation: targeted Codex manifest regressions, the complete `internal/service` plus `internal/repository` unit suites, and the backend production build passed. No deployment or production configuration change was performed.

## Personal Insights clarity and history certification (2026-09-23)

- Simplified the personal dashboard by moving statistical-time, coverage and chart explanations into keyboard-accessible tooltips. Renamed the daily section to `今日实况`, added today's actual amount beside total Token, and hides the daily cache-write item when its value is zero. The heatmap now distinguishes zero use, explicit data gaps, pre-launch dates and future dates.
- Audited production read-only usage history: the billing ledger contains 2,576,729 rows across 112 distinct dates beginning on 2026-06-02, while explicit Insights completeness tracking began on 2026-09-23. Added the operator-only `INSIGHTS_TRUSTED_USAGE_HISTORY_FROM` boundary so audited ledger history can certify Token, amount and idle calendar days without inventing historical success, failure, error or retention facts. The later operational confirmation established 2026-06-01 as the actual production launch boundary.
- Preserved the concrete selected-account provider when an API key belongs to a composite routing group. Historical usage queries also ignore `composite` and `antigravity` fact values and fall back to the concrete account provider, correcting existing `composite:gpt-6-astra` personal logs to the OpenAI provider.
- Widened the bounded desktop canvas for 2K displays and retained the fixed log-column contract. Browser checks at 1920×1080 and 2560×1440 found no horizontal document overflow; the filter row remains intact, Token/performance columns remain separated, and opening the account menu does not move the shell.

Validation: full backend `go test -tags=unit ./...`, React ESLint, TypeScript, 17 files / 58 tests, production build, Docker Compose configuration and whitespace checks passed. Browser fixtures were synthetic; production access was read-only and no deployment was performed.

## Independent CCE release automation (2026-09-23)

- Split the release boundary permanently: GitHub builds linux/amd64 backend and Insights images, pushes only immutable Alibaba ACR images, and archives a combined schema-v2 release manifest. GitHub has no SSH, kubeconfig, or operations-host credential and never starts a deployment.
- Added the repository-owned `deploy/cce/release.py` operations-host command. One invocation validates the manifest, rolls out the backend and current Insights image, runs a single 30-second public observation, writes an audit record, and rolls both Deployments back to their previous Kubernetes revisions on failure.
- Removed the frontend bridge/final sequence from the maintained flow. The direct Insights patch deletes compatibility init containers, the `previous-assets` volume, and old-version mounts, and restores the stable direct-only Nginx ConfigMap.
- Increased the backend HTTP server's bounded graceful shutdown default from 5 to 60 seconds, disabled keep-alives when shutdown starts, and made the timeout configurable through `SERVER_SHUTDOWN_TIMEOUT_SECONDS`. The CCE patch allows endpoint propagation before Kubernetes sends SIGTERM.
- Added release patch/manifest unit tests and CI syntax checks. Pushing a `cce-v<version>` tag now automatically creates the ACR images and manifest; deployment remains an explicit independent operations-host action.

Validation: release unit tests, Python compilation, backend server tests, shell syntax, workflow `actionlint`, whitespace checks, and production-shape dry-run validation passed. No GitHub-to-operations-host connection or cluster credential was introduced.

## Insights official model catalog completion (2026-09-23)

- Replaced the channel-derived Insights model directory with a 77-entry version-controlled allowlist of formal production models across OpenAI, Anthropic/Antigravity, Gemini, DeepSeek/OpenCode and Z.AI. The catalog now includes previously omitted current families and variants while excluding internal names such as `codex-auto-review`.
- Added read-only profiles sourced from official vendor documentation, including descriptions, use cases, context/output limits, modalities and capability declarations. Existing Sub2API pricing resolution still supplies reference prices and remains independent from catalog membership.
- Removed admin profile PUT/DELETE routes, database-backed profile reads/writes, the frontend edit form and the client mutation API. The legacy migration-239 table remains untouched for compatibility; no automatic discovery, scheduled sync or periodic validation was introduced.

Validation: full backend `go test -tags=unit ./...` passed with Git's `bin` on PATH and Go temp/cache redirected to the D drive; catalog/route regressions passed without an `insights_model_metadata` fixture. React ESLint, TypeScript, 17 files / 57 tests and production build passed. No production write, deployment or paid inference request was performed.

## Insights coverage and navigation fixes (2026-09-23)

- Replaced permanently hard-coded partial status on personal usage, usage logs, error logs, daily rollups and the heatmap with range-aware coverage derived from the trusted collection interval and verified daily rollup rows. Partial responses now include the trusted start/observed-through boundary so the UI can explain exactly which earlier dates cannot be certified; unavailable pre-activation history is not fabricated or silently labelled complete.
- Renamed the independent application brand to `AI基础设施看板`. The account dropdown is non-modal so opening it no longer applies document scroll locking or shifts the page.
- Moved the original Sub2API navigation entry out of both user and administrator sidebars and placed it immediately after the announcement bell in the desktop header.
- Gave the personal usage log a fixed column contract, wider Token detail column, separate performance column, borders and subtle group background so token and latency details remain distinct.

Validation: PostgreSQL-backed Insights query tests, handler/route unit tests, React tests/lint/typecheck/build, original Vue targeted tests/lint/typecheck/build and whitespace checks pass. A real 1440×900 local browser session confirmed zero shell movement when the account menu opens, one header Insights link directly after the announcement bell, no sidebar Insights link, and non-overlapping Token/performance columns. Browser fixtures are synthetic.
## Insights PC composition and component redesign (2026-09-23)

Base: 4eb2620eadf2ebe56134ffd6fffeb105222de50e. Branch: codex/insights-pc-polish.

- Implemented the approved PC redesign with three gpt-5.6-sol/high development workers and independent gpt-5.6-sol/high browser validation.
- Added shadcn/Radix composition primitives, cmdk search multi-selects, a timezone-safe range calendar, self-hosted Inter Latin and production license notices. RareUI remains an interaction reference; no RareUI source was vendored.
- Replaced the tall header/filter stack with a 64px header and 60px toolbar. All nine department metrics remain in a 3+6 hierarchy; the main trend fits in the first 1440x900 viewport beside compact model distribution.
- Reworked personal quota/logs and gateway analysis sections without changing their metrics, permissions or backend. Model cards now feed a persistent comparison tray and wide accessible dialog; price units and tier labels preserve source values.
- Fixed date preset anchoring, native-option text compatibility, actual multi-select checked state, dialog focus restoration and keyboard access to automatic refresh. No exports or member drill-down were added.

Validation: 17 test files / 56 tests, ESLint, TypeScript and production build passed. Independent checks cover 24 desktop/theme combinations and final filter, comparison, pagination and ordinary-user flows. Direct local personal/admin-role checks return 200/403. All browser data is synthetic; no production operation or paid inference request occurred. See docs/INSIGHTS_PC_POLISH_VALIDATION.md for evidence and limits.


## Insights v1 bounded release workflow

Source: dee8a2aecd3bd5400df95183cc60c8f9e9822354. Branch: codex/insights-v1.

- Added the registered `cce-clean-image.yml` manual workflow for an exact full source SHA. Dispatch inputs are validated before use and the checked-out commit must equal the requested SHA.
- GitHub builds and pushes separate linux/amd64 backend and Insights images to GHCR with version-plus-full-SHA tags and OCI source, revision and version labels. Only the backend build stages the existing checksum-verified portable Python asset.
- The Insights production build disables source-map emission so the standalone static image does not publish application sources alongside its assets.
- Each pushed image is pulled by immutable digest and exported as a gzip Docker archive with a SHA-256 checksum and JSON transfer manifest. The per-image GitHub artifact is the bounded handoff for the separate ops-host SWR import; the workflow contains no SWR credentials or deployment step.

## Insights v1 and PC visual refinement

Base: af4fa53d3bd6a1bf80924bcbd54212dc983354c3. Branch: codex/insights-v1.

The user approved implementation with gpt-5.6-sol development and independent validation workers, then required a top navigation and a stronger visual design focused on PC. The independent React/TypeScript/Tailwind/ECharts application now provides personal analytics, the system model catalog/comparison, current-department analytics and gateway/user analytics under the original origin's `/insights/` path.

- Sub2API owns JWT, refresh, roles, quota and pricing. Original password, 2FA and existing OAuth completion paths use the shared independent-app navigation helper. Model catalog responses project only permitted model information; ordinary users cannot access admin analytics or other users' logs.
- Request-derived metrics share statistical record time. Retry-aware HTTP/WS collectors preserve actual terminal outcomes and measured timing boundaries; async image/batch/realtime paths retain their separate units. Pure WebRTC and unobservable VAD boundaries explicitly degrade coverage instead of inventing facts or timings.
- Migrations 239/240 add independent call/error facts, user/model daily data, per-source coverage, safe usage archives, model profiles and lifetime observations. Known activity is distinct from a trustworthy true-first anchor. Retention, repeated cleanup, partial coverage, late records and replica gap merging have PostgreSQL regressions.
- PC refinement replaces the sidebar with top module navigation, consistent theme/control/chart tokens, deliberate KPI grouping, readable tooltips, precise small values and platform-timezone display. Department metrics include all nine agreed values. No exports, invented comparisons, member impersonation or model probe calls were added.
- Deployment and validation records are in `docs/INSIGHTS_OPERATIONS.md`, `docs/INSIGHTS_VALIDATION.md` and `insights/DESIGN.md`. Local helper scripts, synthetic fixtures and screenshots stay outside Docker/Git production artifacts.

Validation:

- Backend full `go test -tags=unit ./...` passed. Final changed-package regression (`internal/insights`, `internal/handler`, `internal/server/routes`, `migrations`) passed with real PostgreSQL fixtures; the production embed build passed.
- Original frontend lint, typecheck, production build and the expanded 26-file / 348-test critical suite passed.
- React lint/typecheck/build and 13 files / 43 tests passed. Independent PC checks cover 1366/1440/1920 widths, real same-origin login/API integration, roles, filters, model comparison/profile conflict, chart/theme behavior, unknown data, heatmap selection and refresh retention.
- Docker Compose configuration validates. Docker daemon execution, real external OAuth/2FA callbacks and production data/load were not tested. All database/browser fixtures are synthetic; no paid model inference or production deployment occurred.

## Insights release lint closure (2026-09-23)

- Closed the full uncapped lint inventory by making database row, transaction and test-fixture cleanup handling explicit and validating response contracts at every type assertion.
- Replaced unchecked combined-department response assertions with validated contract errors and removed unused aggregation locals.
- Matched the shared Codex helper to its existing unit-test build tag and removed an unreferenced dialer stub. No production identity behavior, lint rules or actual test cases were removed. CI now reports the full issue inventory without truncation.

## CCE 0.2.7 Selective KLNO Intake

Base: 20d0294f9ec8945ee9933d320a127a3861d1de46.
Target: codex/cce-sub2api-v0.2.7-setup-klno.

- Cherry-picked a7c353703, then adapted model-discovery UA to preserve CPR and
  API-key boundaries. Replaced synthetic header tests with production builders;
  covered bridge originator removal, setup tokens, and ForceCodexCLI precedence.
- Extracted CPR endpoint display from f6cae16ab without turn-state dependencies.
  Added direct/unknown transitions, strict redaction and observation timestamps.
  Kept display freshness separate from quota freshness and scheduling.
- Extracted status-expiry behavior from f0075be5b using a scope-owned shared clock
  rather than one timer per account row. Covered expiry and final-unmount cleanup.
- Added the existing full locale compiler suite to check:i18n and critical CI,
  with fixtures for malformed messages and valid plural/literal syntax.
- Preserved CCE setup scripts, Antigravity fixes, WS provenance guard and the
  extra proxy-log redaction. No token or deployment operations were performed.
- Full-suite validation exposed the pre-existing Gemini keepalive timing race.
  Applied upstream 8f6bbb59c's test-only idle-wait correction (1.2s to 2.2s).
  Repository tests on Windows require Git's bin directory on PATH for sh.

Validation passed:

- Complete service and repository unit suites (195.275s and 5.876s).
- Backend production build: go build ./....
- Frontend lint, typecheck, full locale compilation and production build.
- Critical frontend suite: 23 files, 324 tests.
- Diff whitespace checks and unchanged setup/WS-guard/proxy-log protection files.

Commands:

~~~text
go test -tags=unit ./internal/service ./internal/repository
go build ./...
pnpm run lint:check
pnpm run build
make test-frontend-critical
~~~

## Personal insights live subscription preview (2026-09-24)

- Added subscription-scoped usage aggregation for the latest 60 one-minute buckets. The personal Today response now includes exact request counts and actual cost per subscription minute without changing the existing quota totals.
- Added an abstract recent-hour activity pulse beside each subscription's used amount. It conveys relative activity without exposing minute-level values, while a clearly labelled Vite-only preview fixture supports testing against an older backend.
- Reworked the annual calendar heatmap into one state-aware series. Zero-use hover no longer highlights out-of-scope cells, and the usage-depth and data-state keys now share one legend row.
- Added per-bucket model usage to personal analytics and made the stacked model bar chart the default trend view. The local preview derives a deterministic stack from aggregate model totals when connected to an older backend.
- Applied the requested personal-page copy and card cleanup: `Token用量`, `额度用量`, removed the redundant quota note, and moved reset time beside the subscription name.
- Grouped total, output, daily-average and cache Token metrics before request/activity metrics. Long department, model and API-key log values now stay on one line, truncate visually and expose the full value on hover.
- Simplified the Today token breakdown label from `普通输入` to `输入`. Kept the API-compatible 20-row log pagination, tightened row spacing, and capped the table viewport with a sticky header so the log section no longer dominates the page.
- Fixed `unknown:gemini-3.8-flash-high`: usage SQL had incorrectly excluded `antigravity` even though the model catalog uses it as the production identity. Composite remains excluded until a concrete call-fact provider is available; the frontend also reconciles old-backend `unknown` identities against a unique catalog match during mixed-version previews.

Validation: Insights ESLint, TypeScript, 17 files / 63 tests and production build passed; backend `internal/insights` and `internal/handler` unit suites passed. A 1569x1270 local browser check confirmed both recent-hour pulses, the combined heatmap legend, stable zero-use hover, the default stacked model bars and no horizontal overflow. Preview-only recent and model-bucket data are used only because the connected production backend does not yet expose the new response fields.


## Department consumption classification and overview grouping (2026-09-29)

- Addressed the department-page browser annotations: added a full-width department Token trend above the model Token trend, with independent total/output and line/stacked-bar controls. The request-based model distribution remains adjacent to model consumption.
- Reorganized all nine overview metrics into membership/usage, Token scale, and consumption composition columns, with one primary and two supporting values per column.
- Added actual department and provider-qualified model breakdowns to each department analytics bucket using the existing filtered aggregate rows. Daily/detail hybrid merges retain both breakdowns by identity and recompute model ratios. No migrations or additional database queries.
- Kept category sums conserved when grouping the tail, distinguished same-label identities, retained incomplete bucket labels, and preserved absent breakdowns/null token values as unavailable instead of deriving synthetic production time series.

Validation: Insights lint, TypeScript, all 23 test files / 93 tests, and production build passed. `go test ./internal/insights` passed, including day/week/month bucket merge and classification-conservation regression coverage; PostgreSQL integration tests were skipped because INSIGHTS_TEST_DATABASE_URL is not configured. Browser verification used a loopback-only fixture API and the actual production frontend build, explicitly labelled as example data. Inspected 1366x768, 1440x900 and 1920x1080 desktop layouts in light/dark, confirmed no horizontal document overflow, independent chart controls, and department filtering; browser console reported no warnings/errors. Preview remains at http://127.0.0.1:4188/insights/departments. These changes have not been deployed.

Follow-up: Both department and model Token trends default to stacked bars, with the stacked-bar option before line. The filter toolbar orders date, granularity, department, then model. Rebuilt and reloaded the local preview to verify both default selections and control order. Before committing, reran Insights lint, all 93 tests, production build (including TypeScript), and the backend Insights suite successfully; PostgreSQL integration remains unverified without its test database.
