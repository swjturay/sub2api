# Development Journal

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
