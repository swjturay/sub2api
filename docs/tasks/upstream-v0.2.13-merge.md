# Upstream v0.2.13 merge into cce-deploy

Date: 2026-10-03 (Asia/Shanghai). Scope: source integration and verification.

## Source

- Latest deployment branch before merge: `88c23e57030f2eb07e233e615ffc616897f1c529`.
- Official upstream/main: `b8dece9000c68815a5b867ca5a1e6f236e173905`.
- Includes 40 upstream commits after the previous intake, through v0.2.13.
- Updates include TypeSafe System One, recharge promotion tiers, atomic email
  verification attempts and single-use password-reset tokens, anonymous order
  verification throttling, sanitized Antigravity errors, Grok identity fixes,
  account-priority controls, group-name key sorting, Axios 1.20.0 and settlement
  when an API key is deleted during an in-flight request.

## Compatibility resolutions

Five files had textual conflicts. VERSION follows upstream 0.2.13. Both account
creation and admin updates enforce the existing CPR platform rule and the new
TypeSafe API-key-only rule. Frontend types retain CPR and add TypeSafe. The key
modal retains CCE Codex-first defaults, OS selection, local setup and provider
namespaces; TypeSafe selects the native System One example and excludes Codex
setup. The original CPR relay, quota retry/cooldown, observed billing and
execution-lease files are unchanged by this intake.

The new System One route also needs CCE Insights hooks despite merging cleanly.
Register its HTTP transport, retain the concrete provider, finalize the observed
success, and pass the exact same statistical timestamp to usage recording.
Existing middleware covers rejection, cancellation and upstream error outcomes;
existing HTTP transport instrumentation counts actual sends. Regression tests
exercise the usage-recording path and compare persisted usage and call facts.
The critical frontend CI suite now includes UseKeyModal.

## Migrations

All 294 pre-existing SQL files are byte-identical to the deployment branch.
Upstream adds:

- `241_add_payment_order_bonus_amount.sql`: additive default-zero bonus amount.
- `241_add_typesafe_platform.sql`: expands platform constraints on quotas and
  composite routes to include TypeSafe.

The migration runner keys applied records by full filename and sorts filenames,
so these names coexist with `241_insights_cost_data.sql` and later CCE files.
Do not renumber or rewrite previously applied migration files. The two new SQL
files are retained exactly as upstream supplied them. Production database
migration execution is outside this source merge.

## Validation

Local toolchain: Go 1.27.0, Node 24.19.0, pnpm 10.33.0 with frozen lockfiles.

- Initial backend full unit run passed every package except an existing global
  heap-delta assertion in `TestInflightEstimate_AccountMappingNoDBAndBoundedMemory`
  (8,691,312 bytes versus its 8 MiB threshold). That unchanged test passed three
  isolated repetitions. A full rerun and exact-commit Linux CI are checked in
  the final task result; the test and its threshold have not been weakened.
- Backend `go build ./...` passed.
- CPR/TypeSafe account validation regression passed for creation and admin update.
- System One / TypeSafe / Insights handler regressions passed, including the
  common call-fact and persisted-usage timestamp and request identity.
- Frontend ESLint, typecheck, critical suite (31 files / 476 tests), supplemental
  key modal, priority, payment and recharge tests (4 files / 71 tests), and
  production build passed.
- Insights typecheck, ESLint, 26 files / 104 tests and production build passed.
- CCE release helper: 3 Python tests, Python compilation and shell syntax passed.
- Existing migration byte comparison and Git whitespace checks passed.

No local Docker daemon is installed. Real PostgreSQL/Redis integration and Go
lint are verified by the published commit's GitHub CI; check CI, Insights CI and
Security Scan for that exact SHA before treating the source intake as complete.
No image release, production deployment or live configuration write is included.
