# Development Journal

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
