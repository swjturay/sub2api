# Upstream v0.2.17 merge into cce-deploy

Date: 2026-10-21 (Asia/Tokyo). Scope: source integration and verification.

## Source

- Existing branch: `87676430f40713cbe7fddbd46426fc4b84a288f3`.
- Official [v0.2.17 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.17):
  `060d0160417782cd962a0c7f460f6cd4565d4916`.
- Common upstream ancestor: `01842576b3d5c1e2545b5a99ad555d36e28ed306`.
- 48 incoming commits, including merges.

The release adds Gemini 4 Argon and capability discovery, Gemini reasoning-level
conversion and thinking summaries, configurable minimum output tokens, and
OpenAI API-key web-search emulation. It fixes payment confirmation/refund
handling, Stripe secret transport, protocol compatibility, workspace-aware
imports and Vertex error handling. It also retires GPT-5.5 from direct ChatGPT
subscription accounts and restores inline account filters.

Set the runtime VERSION to 0.2.17; the upstream tag itself still stores 0.2.16.
No production settings, model mappings, account state or deployment is changed.

## Conflict resolution and preserved behavior

Four files required conflict resolution:

- Configuration validation retains the Insights statistics start date check
  and independently validates `gateway.min_output_tokens`.
- `openai_ws_forwarder_ingress.go` retains timezone projection before metadata
  removal and uses the new config-aware compatibility normalizer.
- `openai_ws_v2_passthrough_adapter.go` does the same for both the first frame
  and later response-create frames.
- The upstream deletion of compact-filter tests is accepted by replacing their
  obsolete layout assertions with a behavioral regression: CPR remains directly
  selectable, and choosing it preserves the remaining filters. Include this
  test in critical frontend CI.

Retirement regressions now also exercise CPR accounts and mixed OAuth/CPR
groups through model support, account projection, client-visible catalogs,
scheduler eligibility and compact-fallback selection.
This prevents direct-credential policy from being imposed on the CPR account
type; it does not assert that an actual CPR upstream serves every requested model.

The full backend suite found one stale fork test with three failing cases: it
expected a Lite compaction retry to GPT-5.5 after that model's retirement. Keep
all original instruction-preservation assertions using GPT-5.4, and additionally
assert that retired GPT-5.5 receives no retry across HTTP errors and non-stream/
stream SSE failure terminals. This corrects the fixture without changing or
weakening production retirement policy.

The credential-health observer, management-only scan and conditional quarantine
implementation remain unchanged, as do Insights source/access rules, local
setup scripts, CCE deployment tooling and manual CI triggers. Native CPR relay,
usage evidence and cancellation continue through the existing fork paths.

All 302 SQL migration files are byte-identical to the branch baseline. No new
SQL migration or Go module dependency-version change is introduced.

## Security validation follow-up

The first exact-source Security Scan identified fixes newer than the release's
locked build inputs. Upgrade Go from 1.27.2 to 1.27.3 for
[GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809), synchronizing `go.mod`, all
three compiler Dockerfiles, CI/release assertions and the development guide.

Pin the frontend's transitive `follow-redirects` dependency to 1.16.1 and update
the lockfile for
[GHSA-q3jh-wrc6-jgq8](https://github.com/follow-redirects/follow-redirects/security/advisories/GHSA-q3jh-wrc6-jgq8)
and [GHSA-8r9p-f939-6h3c](https://github.com/follow-redirects/follow-redirects/security/advisories/GHSA-8r9p-f939-6h3c).
These are patch-level fixes, with no new exception or reduced security check.
The existing SheetJS exceptions and their expiry remain unchanged.

## Upgrade boundaries

Direct ChatGPT OAuth/SetupToken accounts no longer expose or schedule GPT-5.5
and its retired variants. Explicit mappings to an available replacement remain
valid; API-key accounts keep their existing policy. Before any future rollout,
review sites that still use GPT-5.5 as a client default or rely on direct
subscription accounts to serve it. This intake does not rewrite client defaults
or production account mappings automatically.

`gateway.min_output_tokens` defaults to 128 and must be at least 16. The merged
WebSocket paths now receive this setting without dropping the fork's timezone
projection. Payment fixes change confirmation and pending-refund behavior using
the existing schema.

## Validation

Initial local checks used Go 1.27.2; final security validation uses Go 1.27.3.
Other tools are golangci-lint 2.14.0, Node 22.21.1 and pnpm 10. Frontend
dependencies use the frozen lockfile.

- Frontend lint/typecheck and critical tests: 40 files / 687 tests passed.
- Full frontend suite: 377 files / 2,938 tests and production build passed.
- Insights lint/typecheck, 27 files / 109 tests and production build passed.
- Go lint reported zero issues.
- Go 1.27.3 vulnerability scanning found no reachable vulnerability; the
  frontend audit passed the unchanged exception policy after the patch update.
- Targeted GPT-5.5 retirement/CPR boundary regressions and the backend build
  with embedded frontend assets passed.
- Local setup and CCE release-helper tests passed; setup shell syntax passed.
- Existing migration bytes and Git whitespace checks passed.

Backend validation uses `go test -tags=unit ./...`,
`go test -tags=integration ./...`, `go build ./...` and a backend build with
embedded frontend assets. Local Docker-dependent database cases skip because
this host has no Docker daemon. CI, Insights CI and Security Scan must be checked
against the published merge SHA; use their manual dispatch entry points if the
upstream skip-CI bookkeeping suppresses automatic runs.

This task does not create release images or deploy the application.
