# Upstream v0.2.14 merge into cce-deploy

Date: 2026-10-07 (Asia/Shanghai). Scope: source integration and verification.

## Source

- Existing deployment branch: `24eca2f43d63d382660aeaea1f6bf3428c152827`.
- Official upstream/main: `3f1a2ea0a760730e3bc528105c00b4ee4f23e469`.
- Eight incoming commits, including merge and VERSION bookkeeping, after v0.2.13.

The intake hardens fresh admin provisioning, rejects forged EasyPay notification
parameter sets, removes client-controlled payment return-URL queries, enables
API-key model discovery for remote Codex catalogs and updates Vue/source-map-js.
The two existing xlsx audit exceptions are extended to 2027-01-06 on the
upstream write-only export rationale; this is an exception renewal, not a patched
xlsx package. No local weakening of that upstream security policy was added.

## Conflict resolution and compatibility

Only `frontend/src/components/keys/UseKeyModal.vue` conflicted, in four Codex
configuration generators. Preserve the CCE feature tables and add
`api_key_model_discovery = true` only for remote catalogs. This applies to native
OpenAI HTTP/WS, Grok through Codex, Composite and all other routed providers.
Do not put Codex options into the native Grok CLI feature table.

Existing CCE remote compaction, image generation, goals, standalone web search,
inline credentials, image-extension header, OS defaults and local setup remain.
The local-file and oversized-catalog fallback modes do not enable remote model
discovery. Tests exercise actual rendered config output and check one features
table, feature preservation and discovery scope across the relevant combinations.

All 296 SQL migration files are byte-identical to the previous branch. The
backend setup/payment changes match upstream. Go module files, CPR native relay
and quota logic, Insights source and CCE release helpers/workflows are unchanged.

## Validation

Local tools: Go 1.27.0, Node 24.19.0 and pnpm 10.33.0. The frontend install uses
the frozen upstream lockfile.

- Setup/payment and existing admin/return-URL regressions passed.
- Generated client configuration: 47 tests passed.
- Frontend lint and typecheck passed; the critical suite passed 31 files / 477 tests.
- Frontend production build, including locale compilation, passed.
- Apple container lifecycle tests, Compose security contract and CCE release
  helper tests passed. Shell syntax and Git whitespace checks passed.
- Full backend `go test -tags=unit ./...` and `go build ./...` passed, including
  the previously intermittent process-wide memory regression without changes
  to that test or its threshold.
- The full frontend run exposed three stale auth-default quota assertions that
  predated this merge: they still expected five platforms although TypeSafe was
  already supported. Their fixtures now include TypeSafe and explicitly verify
  preserving its quota values; production quota code remains unchanged.
- Full frontend suite passed: 341 files / 2,650 tests.

The local host has no Docker daemon. Exact-source GitHub CI supplies real Linux,
PostgreSQL/Redis integration, Go lint, Insights CI and security scans. Their
status must be checked for the published merge commit.

## Upgrade boundaries

Fresh automatic installations now generate a random admin email when omitted
and reject an explicit invalid login email or password outside 8-72 bytes.
The validation runs only when a new admin is needed; existing users/admins are
not changed or prevented from starting by this check.

EasyPay callbacks with nonstandard parameters are rejected. The upstream
reconciliation path remains available for legitimate payments from variants that
add extra notification fields. Client query parameters are no longer preserved
in payment return URLs.

This task does not build release images, change production resources or deploy
v0.2.14. The preceding operations task
recorded an independently published v0.2.13 release.
