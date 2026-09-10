# CCE Sub2API v0.2.4 migration

Date: 2026-09-10

## Provenance

- Target branch: `swjturay/sub2api:cce-sub2api-v0.2.4`.
- Upstream base: `Wei-Shaw/sub2api:v0.2.4`, commit
  `5de5e2bed035d43591a2e10e51f420ef6a84eb98`.
- Customization source: `cce-sub2api-v0.2.3`, commit
  `5474ce9e5a6e13792c77f3997a8df238d6012a7e`.
- Replayed 12 of the 13 source-branch commits with `cherry-pick -x`, including
  regression coverage and the historical v0.2.3 migration record.
- Omitted `2086ddc1c` (close plugin ZIP before Windows rename): upstream commit
  `335fcdc1d` already implements this fix. The plugin installer and its tests
  remain byte-for-byte identical to upstream v0.2.4.

## Preserved Customizations

- One-command local setup for Codex, Claude Code, and OpenCode, preserving
  unrelated configuration and creating backups.
- Simplified Codex provider configuration, optional native WebSocket setup,
  Windows PowerShell host retention, and bounded/retried bootstrap downloads.
- Codex picker alias/dedicated-model policy, group filtering, final ETags,
  and source-cache isolation.
- HTTP concurrency admission status and `Retry-After` preservation, while
  retaining the Anthropic Messages heartbeat behavior.
- Manually dispatched CCE image/archive workflow, verified portable Python,
  and OCI provenance labels; default version is now `0.2.4-clean-codex`.

## v0.2.4 Adaptation

Resolved the UseKeyModal configuration-template conflict without losing
upstream MiniMax client tabs or model defaults. MiniMax Codex guidance now
matches the fork's direct-key, no-extra-catalog configuration.

The custom OpenCode exporter and Python setup helper now classify MiniMax
models into `shared-ai-minimax` using `@ai-sdk/openai-compatible`, for both
MiniMax and Composite groups. MiniMax discovery failure uses the upstream
MiniMax model list rather than falling back to OpenAI models. Regression tests
were observed failing before the adaptation and passing afterward.

The upstream database/Ent changes, migration 237, dependency manifests and
lockfiles remain unchanged. Upstream cancellation and HTTP/2 keepalive changes
are retained; no old private transport bridge was reintroduced.

## Verification

Environment: Windows, Go 1.27.0, pnpm 10.33.0.

- Frozen-lockfile dependency installation, frontend lint, and frontend build:
  passed, including TypeScript and locale completeness checks.
- Targeted frontend setup/model/device tests: 54 passed across four files.
- Python setup tests: 13 passed, including Windows bootstrap tests and new
  MiniMax provider/fallback coverage.
- Focused handler/service tests for Codex, concurrency, plugins, group models,
  slot acquisition, Claude probe context, and client cancellation: passed.
- `go test -p 2 ./internal/handler ./internal/service ./internal/server/...
  ./migrations`: all selected packages passed.
- Linux amd64 cross-build with `CGO_ENABLED=0`, `-tags embed`, and `-trimpath`:
  passed with the built frontend embedded.
- `git diff --check v0.2.4`: passed.

Full frontend run: 1992 passed, 2 failed. Both failures were reproduced in a
clean detached worktree at the exact upstream v0.2.4 commit:

- `GroupsView.codexManifest.spec.ts`: missing active Pinia in the test setup.
- `ChannelMonitorView.grok.spec.ts`: expects 8 provider buttons although the
  upstream MiniMax addition makes 9.

Those unrelated upstream tests were not modified. Full-repository backend
integration tests requiring external services were not run locally.

This change creates and publishes a source branch only. No image build,
registry sync, production rollout, or live database change was performed.
