# Upstream v0.2.21 merge into cce-deploy

Date: 2026-11-12 (Asia/Shanghai); started 2026-11-11. Scope: source integration and verification.

## Source

- Branch baseline: `480facc6082d790c2bb0d9e45f1139c45df86cb1`.
- Official [v0.2.21 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.21):
  `10ba8f9b4dc2e6023d4a9879ab77b21841babd10`.
- Common upstream ancestor: `dece26153621959ccd7635814f4b2d08304e681a`
  (v0.2.19); 194 incoming commits, including v0.2.20.
- The release tag still contains VERSION `0.2.20`; this branch reports `0.2.21`.

## Upstream intake

Includes MCP/OAuth management, expanded scoped administrator permissions,
registration email blacklist, image-size routing and GPT Image 2.6, Claude
Fable/Mythos 5.2 catalogs, OpenAI 30m/12h cache-write accounting, adaptive ingress
memory budgeting, bounded response/session storage and upstream read/write/drain
timeouts. It also includes Responses/WebSocket failover and continuation fixes,
Codex page-context handling, Antigravity quota improvements, key-list query
indexing, asynchronous usage-recording changes and dependency/security updates.

## CCE compatibility decisions

- Preserve CPR native compressed wire bytes and cancellation/received-usage
  settlement. Ingress memory accounting retains both compressed and decoded
  allocations until the handler ends; normalization drops stale wire storage
  before refunding owned allocations. Borrowed preread buffers remain owned by
  their original scope. Regression tests verify exact wire bytes and charges.
- Preserve management-only CPR quota reads, shared/jittered direct OAuth quota
  caching, snapshot-based credentials/identity/proxy resolution, and scoped
  authorization before credential access. Adapt upstream scoped tests to the
  snapshot boundary; do not reintroduce synthetic inference quota probes.
- Preserve Insights actual-send observations, explicit final statistical time,
  permission boundaries and `codex_observation` storage alongside the new TTL
  token columns. Both usage INSERT paths now have 65 columns/arguments.
- Keep CCE local setup, OS selection, default legacy Codex authentication,
  remote compaction and multi-provider OpenCode configuration. API-key mode
  remains selectable; new Anthropic model metadata uses the existing
  `shared-ai-anthropic` provider. Keep both fork and upstream regression cases.
- Preserve CPR WebSocket bare-error turn completion together with upstream's
  configurable first-token timing. Merge Wire initialization and cleanup for
  both Insights/CPR and upstream MCP/balance/session services.
- Keep the fork's existing disabled native Codex Images endpoint gate. New
  upstream tests that require that endpoint use the same explicit feature-gate
  helper as existing direct-image tests; parser, API-key and billing cases stay
  covered separately. This merge does not enable direct OAuth image transport.
- Insights, CCE/GitHub/CodeArts release controllers and local setup scripts
  remain unchanged. Upstream CI action-version updates retain fork concurrency
  and sync workflows. No fork-owned file is deleted.

## Go security follow-up

The post-merge `govulncheck` flags Go 1.27.3's standard library for
[GO-2026-6859](https://github.com/golang/vulndb/blob/master/data/osv/GO-2026-6859.json)
(CVE-2026-97054), an HTTP/1 MaxBytesReader header race that can crash a server.
The official Go vulnerability record identifies Go 1.27.4 as the fixed version.
Update the Go module minimum, default backend image, CI version assertions and
both CodeArts validation images together to 1.27.4. The release-controller
logic is unchanged; no vulnerability exception is added for this finding.

The complete backend unit target now runs the existing bounded-heap test alone
in a fresh process, matching the CCE validation gate. The first all-in-one run
measured 8,503,608 bytes against its unchanged 8,388,608-byte bound because other
tests share the process. All other unit tests still run, and the isolated test
is mandatory; no assertion or test inventory is removed.

## Migration boundary

All existing SQL migration files are byte-identical to the branch baseline.
New files are `249_mcp_oauth.sql`,
`249_add_usage_logs_api_key_cost_index_notx.sql` and
`250_openai_cache_write_ttl.sql`. The duplicate numeric prefix is upstream's
file naming; migrations are tracked by full filename. The usage-cost index
requires non-transactional `CREATE INDEX CONCURRENTLY`, handled by the migration
runner. Production upgrade planning must account for its duration on large
usage tables. This source-sync task does not run production migrations, build
release images or change any running service.

## Verification

Local validation:

- Go 1.27.4: the complete `make test-unit` inventory passes, including the
  unchanged isolated bounded-heap assertion; scoped quota/image-gate adaptation
  regressions also pass independently. `golangci-lint` reports 0 issues,
  `govulncheck` reports no reachable vulnerabilities, and `go build ./...` passes.
- Frontend: lint, typecheck, 40 critical files / 703 tests and production build
  pass. The full 391-file / 3,185-test run passed every file except the merged
  UseKeyModal expectations; after aligning those with CCE defaults, all 53 tests
  in that file pass on rerun. Locale compilation also passes.
- Production-dependency frontend audit passes the existing exception policy.
- Insights: lint, typecheck, 29 files / 118 tests and production build pass.
  Isolated PostgreSQL Insights tests pass on Go 1.27.4; handler and route tests
  also pass against the isolated test database.
- CCE release tests (4), CodeArts controller tests (5), local setup tests (20,
  including 3 Windows-only skips), and backend test-runner tests (6) pass.
  Deployment shell/security/resource checks pass locally.
- Existing 305 SQL migrations are byte-identical. Insights and local setup
  trees are byte-identical. The only baseline file deletion is upstream's
  removed sponsor image. The new SQL migration's existing blank EOF line is
  retained to keep its upstream checksum unchanged.

The Linux CI jobs validate the exact published source, including the real
Docker/PostgreSQL repository integration suite and release-matrix helpers.
Those two checks cannot run fully on this Mac: no Docker daemon is installed,
and `/bin/bash` is Apple's Bash 3.2 rather than the Bash 4+ required by the
release helper. Production services are not used as a substitute test target.
Both frontend builds retain their existing chunk-size warnings.
