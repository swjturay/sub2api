# Development Journal

## Insights v1 and PC visual refinement

Base: af4fa53d3bd6a1bf80924bcbd54212dc983354c3. Branch: codex/insights-v1.

The user approved implementation with gpt-5.6-sol development and independent validation workers, then required a top navigation and a stronger visual design focused on PC. The independent React/TypeScript/Tailwind/ECharts application now provides personal analytics, the system model catalog/comparison, current-department analytics and gateway/user analytics under the original origin's `/insights/` path.

- Sub2API owns JWT, refresh, roles, quota and pricing. Original password, 2FA and existing OAuth completion paths use the shared independent-app navigation helper. Model catalog responses project only permitted model information; ordinary users cannot access admin analytics or other users' logs.
- Request-derived metrics share statistical record time. Retry-aware HTTP/WS collectors preserve actual terminal outcomes and measured timing boundaries; async image/batch/realtime paths retain their separate units. Pure WebRTC and unobservable VAD boundaries explicitly degrade coverage instead of inventing facts or timings.
- Migrations 239/240 add independent call/error facts, user/model daily data, per-source coverage, safe usage archives, model profiles and lifetime observations. Known activity is distinct from a trustworthy true-first anchor. Retention, repeated cleanup, partial coverage, late records and replica gap merging have PostgreSQL regressions.
- PC refinement replaces the sidebar with top module navigation, consistent theme/control/chart tokens, deliberate KPI grouping, readable tooltips, precise small values and platform-timezone display. Department metrics include all nine agreed values. No exports, invented comparisons, member impersonation or model probe calls were added.
- Deployment and validation records are in `docs/INSIGHTS_OPERATIONS.md`, `docs/INSIGHTS_VALIDATION.md` and `insights/DESIGN.md`. Local helper scripts, synthetic fixtures and screenshots stay outside Docker/Git production artifacts.

Validation:

- Backend full `go test -tags=unit ./...` passed. Final changed-package regression (`internal/insights`, `internal/handler`, `internal/server/routes`, `migrations`) passed with real PostgreSQL fixtures; the production embed build passed.
- Original frontend lint, typecheck, production build and the expanded 26-file / 348-test critical suite passed.
- React lint/typecheck/build and 13 files / 43 tests passed. Independent PC checks cover 1366/1440/1920 widths, real same-origin login/API integration, roles, filters, model comparison/profile conflict, chart/theme behavior, unknown data, heatmap selection and refresh retention.
- Docker Compose configuration validates. Docker daemon execution, real external OAuth/2FA callbacks and production data/load were not tested. All database/browser fixtures are synthetic; no paid model inference or production deployment occurred.

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
