# CCE Sub2API v0.2.5 migration

Date: 2026-09-16

## Provenance

- Base: upstream `Wei-Shaw/sub2api:v0.2.5`, commit
  `86f93c28ee34cc74b629dafb748bd5ac5ca8c5ea`.
- Target: `swjturay/sub2api:cce-sub2api-v0.2.5`.
- Custom source: `cce-sub2api-v0.2.4`, commit `73bdea93ac499e52d6a89f421ba2d52a0a891a35`.
- The complete `v0.2.4` customization delta was merged from its common
  upstream `v0.2.4` base, preserving all 28 customized paths and avoiding
  duplicate cherry-picks.

## Preserved Features

- Codex, Claude Code, and OpenCode one-command local setup with configuration
  preservation, backups, Windows download retries, and native Codex options.
- Codex picker alias/dedicated-model filtering, group allowlists, pinned account
  discovery, final ETags, and cache isolation.
- MiniMax OpenCode classification and fallback catalogs for native and composite
  groups, using `@ai-sdk/openai-compatible`.
- HTTP concurrency `Retry-After` behavior, client-disconnect handling, and
  Windows plugin archive handling.
- CCE image/archive workflow with OCI provenance; default image version is now
  `0.2.5-clean-codex`.

## Upstream Compatibility

The merge retains all upstream `v0.2.5` changes, including MiniMax support,
HTTP/2 stream keepalive, Image 2.5 support, Grok media eligibility, proxy
partial updates, client cancellation fixes, and migration 237. The upstream
`opencode_go` Codex alias was retained alongside the custom OpenCode provider
classification.

## Verification

- Python setup suite: 13 tests passed.
- Focused frontend setup/model/device suite: 54 tests passed.
- Focused backend handler/service suite covering Codex, concurrency, plugins,
  group models, Claude probes, and client cancellation: passed.
- Expanded backend packages `internal/handler`, `internal/service`,
  `internal/server/...`, and `migrations`: passed.
- `git diff --check v0.2.5`: passed.

The full frontend suite retains two known upstream failures: a missing Pinia
test setup and a provider-count assertion not updated for MiniMax. Both failures
were reproduced on a clean `v0.2.5` worktree and are unrelated to this merge.

No production deployment or database migration was performed for this branch.
