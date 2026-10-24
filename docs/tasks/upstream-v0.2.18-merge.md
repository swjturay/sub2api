# Upstream v0.2.18 merge into cce-deploy

Date: 2026-10-24 (Asia/Tokyo). Scope: source integration and verification.

## Source

- Branch baseline: `768b8ecf893ea8aaf8a0dbcbab07b83e2c0c78b9`.
- Official [v0.2.18 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.18):
  `ac1b8f1271be0d674fa6f074403c1410f30a3a81` (peeled tag commit).
- Common upstream ancestor: `060d0160417782cd962a0c7f460f6cd4565d4916`.
- 45 incoming commits, including merges. Runtime VERSION is corrected to 0.2.18;
  the upstream tag still contains 0.2.17.

This release adds Vertex total/daily/weekly account quotas and Gemini image
permission enforcement. It improves scheduler snapshot reuse, model-mapping
signatures, lazy Antigravity probing and quota billing SQL. Fixes include
Anthropic/Gemini interrupted-stream billing, protocol termination, Cloudflare
1010 classification, Gemini schema/cache usage, DeepSeek images, Grok Fast
pricing, asynchronous image download restrictions, and Codex overload retry
handling. Account platform/status filters now persist across refreshes.

## Reconciliation and retained features

Six files had textual conflicts: both compatibility handlers, the Codex gateway
constants and version-sync test, and the frontend package/lock files.

- Both Chat Completions and Responses handlers use upstream's shared usage
  submission for success and partially observed failed streams. Preserve the
  fork's `StatisticalAt` value and requested reasoning metadata. Failures remain
  failures in Insights; a client-disconnected result is cancelled even when it
  has billable usage. Actual handler tests cover complete, interrupted and
  cancelled requests, exactly one settlement and one statistical time.
- Canonical direct Codex identity follows upstream's CLI format and paired
  TUI-to-CLI normalization. Remove the previously appended TUI trailer from
  canonical CLI constants and builders together. Keep the fork's missing-
  originator normalization, ForceCodex/account precedence, separate browser and
  Codex management clients, and CPR identity boundary.
  The first full service test run found one stale canonical-UA builder assertion
  requiring the old trailer; align that fixture with upstream's CLI format.
- Keep the exact follow-redirects 1.16.1 security override and accept DOMPurify
  3.4.17. Go 1.27.3, compiler image defaults, manual validation triggers and
  security exception policy remain unchanged.
- CPR-specific relay, accounting, quota refresh, cooldown and credential-health
  source files are unchanged. The shared scheduler retains CPR plan metadata.
  A regression exercises the new memory cache with CPR model/subscription
  policy and verifies immediate quarantine and explicit recovery visibility.
- Insights frontend, read-access rules, local client setup scripts and CCE
  release helpers remain unchanged. Existing CPR/native relay and Insights
  regressions remain part of backend and frontend validation.

## Database and future deployment boundaries

All 302 existing SQL migration files are byte-identical to the branch baseline.
The only new SQL file is `246_enable_gemini_image_generation_groups.sql`.
It enables image generation for existing Gemini and Antigravity groups; it
deliberately leaves composite groups unchanged. Before a future deployment,
confirm any composite group serving Gemini images has its image-generation
switch enabled. Grok 4.7 Fast/build-fast pricing changes to the official Fast
price (twice the standard variant).

No production settings, account state or Kubernetes resources are modified.
This task does not create release images or deploy the application.

## Validation

Use Go 1.27.3, golangci-lint 2.14.0, Node 22.21.1 and pnpm 10 locally.
Both JavaScript applications install from frozen lockfiles.

- Frontend lint/typecheck and critical suite: 40 files / 688 tests passed.
- Full frontend suite: 377 files / 2,942 tests passed.
- Insights lint/typecheck, 27 files / 109 tests and production build passed.
- Compatibility handler regressions passed for normal, interrupted and
  client-cancelled streams, with usage/statistics consistency assertions.
- Go vulnerability scanning found no reachable vulnerability. Frontend audit
  passed the unchanged exception policy; no waiver was added.
- Local setup: 20 tests (three Windows-only cases skipped); CCE helpers: three
  tests passed. Shell syntax and release Python compilation passed.
- Existing migration bytes and Git whitespace checks passed.

Backend full unit/integration checks, lint and builds are recorded in the
validation logs. Local Docker-dependent database tests require Linux CI because
this host has no Docker daemon. The upstream release-matrix shell tests also
require modern Bash (macOS /bin/bash 3.2 lacks the lowercase substitution).
Verify CI, Insights CI and Security Scan against the published source SHA;
manually dispatch if upstream skip-CI bookkeeping suppresses push validation.
