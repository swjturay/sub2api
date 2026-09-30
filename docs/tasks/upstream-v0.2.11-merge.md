# Upstream v0.2.11 merge into cce-deploy

Date: 2026-09-30. Scope: source integration and verification; no production
deployment, image publication or live configuration changes.

## Source and retained work

- Official [v0.2.11 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.11),
  commit `96f4c115c9749078f90cbf210a01d39baf3f53b6`.
- Existing CCE head: `d130858c7f1472b0b6a543bf94efc16ac861ac01`.
- Previously uncommitted CPR quota retry repair saved first as `d7685f2c7`;
  see [its task report](cpr-quota-retry-fix.md).
- Merge base with upstream: `4c00df2e0183e2c70b7fa8ba45914205e36aad0c`.
  This intake therefore includes v0.2.10 and v0.2.11 changes, not later main.

The release adds model support and pricing, remote Codex catalogs, Claude reset
credit query/redemption, API key creation limits, balance reservation, Claude
Code-only group fallback, composite WS routing and streaming usage fixes.

## Resolution decisions

Eight files needed textual resolution. Keep both upstream behavior and CCE
customizations where they overlap:

- `backend/cmd/server/VERSION`: use `0.2.11`; the tag's file was still `0.2.10`,
  which would incorrectly identify builds from a non-tagged merge commit.
- `wire_gen.go`: retain Insights handlers; reuse the earlier idempotency
  coordinator for Claude reset. Regeneration with Wire v0.7.0 matches exactly.
- OpenAI gateway test: retain CCE upstream-header assertions and upstream Grok
  test server responses.
- Account status test: retain shared countdown regressions and Sonnet 5.5 cases.
- UseKeyModal and tests: retain Codex defaults, automatic OS selection, inline
  API key, web/image capabilities, local installer and `shared-ai-*` OpenCode
  providers. Add upstream remote/file model catalog mode and Sonnet 5.5 metadata
  within the existing provider namespaces. Local catalog TOML uses `~/` on
  Windows as well; `%userprofile%` remains a displayed file location only.
- English/Chinese dashboard strings: keep remote/local catalog instructions
  consistent with CCE's inline-key configuration.

No migration/schema, Go module or frontend lockfile changes are introduced by
this upstream increment. CCE CPR raw relay, conversion, local token counting,
cookie isolation, call IDs, model metadata, safety/routing observation and
Insights remain present. Existing model mapping remains supported.

## Balance reservation compatibility

The admission estimate is a temporary concurrency/balance guard, not billable
usage. CPR billing still uses observed usage and its distinct attempt identity.
HTTP Responses, Chat and Messages preserve the reservation context after the
CPR capture wrapper returns. WS submits usage with its reserved connection
context. The merged task wrapper acquires one reference per queued billing task
and releases it after settlement, drop, synchronous fallback or panic.

Additional regressions cover CPR quota failures on all three protocols in both
JSON and SSE modes, and cancellation followed by queued usage settlement. They
assert that handler completion cannot release the reservation while that usage
task is still running. Existing tests cover mandatory billing fallback.

## Validation

- Backend full unit suite: `go test -tags=unit -p 2 ./... -count=1` passed.
- Added CPR/reservation regressions plus existing retry/usage-submit tests:
  `go test -tags=unit ./internal/handler ./internal/service -run
  'Test(CPR|WrapUsageRecord|ReserveInflight|OpenAIGatewayHandlerSubmit|GatewayHandlerSubmit)'
  -count=1` passed.
- Frontend full suite: 339 files, 2,621 tests passed.
- Frontend production build (including i18n and Vue type compilation) passed.
- Wire regeneration using `go run github.com/google/wire/cmd/wire@v0.7.0`
  passed without module changes or generated-file differences.
- Root `tools/validate-repo.ps1` passed; its Windows directory-symlink test
  skipped because this host lacks that privilege.
- Final `go build -p 2 ./...`, Go lint (zero issues), frontend `lint:check`,
  `typecheck` and staged/working-tree whitespace checks passed.

Local Go uses 1.27.0; Go lint uses 2.13.0 over tracked package directories.
The pre-existing ignored `backend/tmp/models-preview` scratch program is
excluded from lint. Git Bash is on PATH for shell-backed tests on Windows.
Local Docker is unavailable, so local unit success is not evidence of real
PostgreSQL integration coverage. CI and Insights CI on the published merge
commit provide Linux, Docker, Redis and real PostgreSQL verification; their
results must be checked separately before reporting the intake complete.

## Deployment considerations

- `billing.inflight_reservation.enabled` defaults to true, with a 900-second
  TTL and 8,192 estimated output tokens when a request does not specify a
  maximum. Balance-mode concurrent requests may now be refused based on the
  estimate; the first request is still admitted under the existing balance
  check. Subscription mode is unaffected. Unpriced/Redis-unavailable estimates
  fail open by default. Inspect effective production settings before release.
- API key creation defaults: 200 non-deleted keys per user, 60 creates per user
  per hour; deleting a key does not refund the hourly count. Either configured
  limit can be disabled with zero.
- Remote Codex catalog configuration targets clients supporting that upstream
  capability (documented as 0.156+); file mode remains available, including the
  large-manifest fallback. The CCE one-click installer retains its existing
  configuration behavior.
- Claude reset redemption is an explicit administrator action, not an
  automatic side effect of this merge or an automatic CPR quota reset.

There is no production rollout in this task. A later deployment should use the
validated source SHA and the existing CCE staged release and health checks.
