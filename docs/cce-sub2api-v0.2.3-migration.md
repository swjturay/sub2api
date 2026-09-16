# CCE Sub2API v0.2.3 migration

Date: 2026-09-08

## Provenance

- Target: `swjturay/sub2api`, branch `cce-sub2api-v0.2.3`.
- Base: upstream `Wei-Shaw/sub2api` tag `v0.2.3`, commit
  `8fa67d477d6651a744754392a8982ea589c26ae6`.
- Source customizations: `cce-sub2api-v0.2.1` at
  `78825d4a871ab4091784981e9a974f91164e6c86`, based on upstream `v0.2.1`.
- All 11 customization commits were replayed in order with `cherry-pick -x`.
  The upstream release tag is `v0.2.3`, not `v2.0.3`.

## Preserved Features

- One-command local setup for Codex, Claude Code, and OpenCode, with model
  discovery, configuration preservation, backups, and optional native Codex WS.
- Simplified Codex provider configuration and Windows PowerShell host retention.
- Bounded/retried Windows downloads and verified portable Python staging.
- Codex picker alias preference and dedicated-model hiding, without changing
  inference IDs or removing resume metadata.
- HTTP status and `Retry-After` preservation for concurrency admission failures.
- Plugin archive closure before rename on Windows.
- Manually dispatched CCE image build/archive workflow and OCI provenance labels.
  Its default runtime version is now `0.2.3-clean-codex`.

## Upstream Adaptation

The Codex picker commit conflicted with upstream's model discovery refactor.
The port uses `OpenAIModelsResponse` and retains pinned discovery precedence,
account mappings, fallback behavior, and the new group model allowlist.
Picker filtering runs after those policies and before final ETag comparison.
The existing generic response writer is reused without applying picker policy
to ordinary OpenAI model lists. Cached source manifests remain unmodified.

Eight additional handler scenarios exercise configured, pinned, scheduled, and
fallback discovery, with and without an alias-only allowlist. They cover mapping
order, pinned precedence, final ETags/304s, and upstream cache reuse.

Upstream database migrations (including repair migration 236), generated Ent
code, dependency manifests, and lockfiles remain unchanged. No private transport
bridge or other features outside `cce-sub2api-v0.2.1` were imported.

## Verification

On Windows with Go 1.27.0 and pnpm 10.33.0:

- `pnpm install --frozen-lockfile`, `pnpm lint:check`, and `pnpm build`: passed.
- Targeted frontend setup/model/device tests: 51 passed across four files.
- `python -m unittest discover -s frontend/public/scripts/tests -v`: 11 passed.
- Handler/service tests matching
  `Test(Codex|.*Concurrency.*|.*PluginPackage.*|.*Acquire.*|.*Group.*Models.*)`:
  passed with `-count=1`, including the new handler scenarios.
- Full handler package, routes, middleware, and migration tests: passed in the
  broad backend run.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -p 2 -tags embed -trimpath ...
  ./cmd/server`: passed with the built frontend embedded.
- `git diff --check v0.2.3`: passed.

Full suites are **not entirely green**. Baseline checks used a separate, pristine
worktree at the exact upstream tag:

- Frontend: 1917 passed, one failed. `GroupsView.codexManifest.spec.ts` lacks an
  active Pinia instance; the identical failure reproduces on pristine upstream.
- Backend: `TestContentModerationRuntimeSnapshotRefreshFailureKeepsStaleConfig`
  intermittently times out. Five-run targeted checks reproduce the timeout on
  both branches; this unrelated test and implementation were not changed.
- Windows cannot execute the `cmd/profit-preview`, `internal/handler/dto`, and
  `internal/handler/quotaview` test binaries reliably (missing executable or
  `exit status 0xffffffff`). The same failures reproduce on pristine upstream.
- Repository backup tests initially failed because `sh` was absent from PATH.
  Adding Git's `bin` and `usr/bin` to the test process PATH makes the entire
  repository package pass without source changes.

No image workflow, registry publication, production deployment, or live database
migration was requested or performed. Linux runtime/integration validation remains
a release-stage check, separate from this branch migration.
