# Upstream v0.2.15 merge into cce-deploy

Date: 2026-10-13 (Asia/Shanghai). Scope: source integration and verification.

## Source

- Existing deployment branch: `9a1800821fb8cc4e1c4d39a3a6dcb21e2647c837`.
- Official [v0.2.15 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.15):
  `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`.
- Common upstream ancestor: `3f1a2ea0a760730e3bc528105c00b4ee4f23e469`.
- 145 incoming commits, including merges, since the preceding intake.

The release adds Cline and Command Code through shared platform/provider
catalogs, refreshes group pricing for later WebSocket turns, retains image
input usage, fixes protocol conversion and OAuth web-search history, and fixes
frontend request races. Adopt upstream Go 1.27.2, golangci-lint 2.14.0 and the
updated Go dependencies. Set `backend/cmd/server/VERSION` to 0.2.15 because the
release tag itself still stores 0.2.14.

## Conflict resolution and compatibility

Six files required explicit conflict resolution:

- CI uses upstream golangci-lint 2.14.0 while preserving the fork's complete
  diagnostic output flags and setup/deployment jobs.
- The OpenAI handler retains both the new platform-domain import and Insights.
- Token counting uses provider routing profiles while preserving local
  estimates for direct OAuth/setup-token and CPR accounts.
- Auth-default tests use the complete platform catalog and retain TypeSafe
  value-preservation checks.
- Client configuration tests retain CCE feature and discovery assertions plus
  upstream goals/WS checks. New-platform cases cover the CCE setup selector and
  remote/file catalogs on Unix and Windows.
- Account platform types adopt upstream's extensible catalog while retaining
  the CPR account type.

The full frontend suite exposed a new upstream compact-filter expectation that
omitted the fork's CPR option. Include CPR and exercise selecting it; production
filter behavior remains intact. Remove one duplicate critical-test path created
by automatic merging of the Makefile.

Insights source and read-access rules, CCE release helpers/workflows, and the
Python 3.8-compatible setup helper are byte-identical to the preceding branch.
CPR relay, cancellation, quota and received-usage semantics remain covered by
the backend suite. The two new platform catalogs use the existing gateway
handlers and therefore retain Insights recording.

## Migrations and upgrade boundaries

All 297 existing SQL migrations remain byte-identical. Upstream adds
`242_drop_platform_check_constraints.sql`; the full filename is distinct from
`242_insights_cost_account_contributors.sql`. The runner tracks full filenames,
so neither file is renamed. Platform membership now belongs to application
validation; monitor provider constraints remain intact.

A local PGlite execution verified that the new migration preserves existing
rows, permits Cline/Command Code quota and composite-route rows, can run twice,
and leaves the monitor provider constraint enforced. It does not exercise a
production database. Once the migration runs, rolling back an application image
does not restore these constraints; old application code cannot manage the new
platforms. Review platform usage before any later application rollback.

This intake does not publish image tags, run the ACR release workflow or change
production resources. A later release must build and deploy the reviewed source
through `deploy/cce/README.md`.

## Validation

Local validation uses Go 1.27.2, golangci-lint 2.14.0 and Node 22.21.1/26.11.0.
Frontend dependencies were installed with the frozen lockfile.

- Frontend lint, typecheck, critical tests and production build passed.
- Full frontend suite passed: 366 files / 2,860 tests. Generated client
  configuration tests passed: 51 tests, including both new platforms.
- Insights typecheck, lint, 109 tests and production build passed. Three tests
  timed out while full frontend/Go work competed for local CPU; rerunning with
  two workers passed without changing code or timeout limits.
- Full backend unit and integration-tag suites, `go build ./...`, and the
  embedded-frontend backend build passed; Go lint reported zero issues.
  Docker/database-dependent integration cases skip locally and still require
  the Linux CI checks below.
- CCE release helper and local setup tests passed, as did Apple container
  lifecycle, Compose security, gateway-environment, runtime-resource and Caddy
  checks.
- SQL migration immutability and local execution checks passed.

Local Docker-dependent checks require the exact-source GitHub CI environment.
The local host has no Docker executable or daemon. Release-matrix shell tests
also require Linux/modern Bash: macOS `/bin/bash` 3.2 cannot expand the existing
`${owner,,}` expression. Check CI, Insights CI and Security Scan for the
published merge commit before creating release images.
