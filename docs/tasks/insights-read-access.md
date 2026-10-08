# Insights read access

Date: 2026-10-08. Implementation based on `c702a3880476a30668e7708c6606ece73edf9f88`.

## Delivered behavior

Ordinary users can receive company-wide department, gateway and cost read
access through one switch in user management. All existing data, fields and
filters remain visible; cost edits, attribute grants and system administration
still require admin identity. No new system role, users column, department
scope, cost-specific grant or generic RBAC framework is introduced.

Migration 244 inserts the protected `insights_access` definition into existing
extension tables. `enabled` is the sole granting value; missing/empty values
default off. The definition cannot be edited/deleted through generic APIs,
and DingTalk configuration, definition repair and sync writes reject the key.
A conflicting definition stops migration for operator review rather than
silently adopting existing data as permission.

Reads use current database authorization; admin attribute list caches do not
participate. `/auth/me` exposes effective access. Browser permissions refresh
at 60-second intervals even when chart refresh is disabled, and revalidate
before content returns after foregrounding. Failed checks clear protected
content; unmounted requests cannot restore old data. Requests authorized
before revocation are not forcibly terminated.

## Migration and release

The change requires both backend and frontend release. Existing admin read
URLs remain available for compatibility. No existing user is promoted or
demoted automatically. An operator may grant read access then downgrade a
user's system role; the user edit form saves attributes before the role so
attribute failure cannot remove the user's prior access.

Application rollback preserves attribute data but old application versions do
not recognize viewer access. Do not automatically restore admin identity as
a rollback workaround. This source task does not deploy production or select
users to migrate.

## Validation

- `go test -tags=unit ./...`: passed, 58 packages.
- Focused permission, route-boundary, DingTalk protection and `/auth/me`
  contract tests passed. Tests exercise grant/revoke without role changes,
  denied writes, malformed attributes and failed authorization reads.
- `go test -tags=integration ./...`: command passed, but the repository's
  database harness skipped Docker-backed tests on this machine because Docker
  is unavailable. Remote CI is required for that part of validation.
- `golangci-lint v2.13.0` over `cmd`, `ent`, `internal`, `migrations`, `pkg`:
  zero issues.
- Insights lint, typecheck, test and production build passed; permission tests
  cover the viewer cost table, role-independent cache scope, revocation polling,
  foreground verification and failed identity checks.
- Main frontend `make test-frontend`, the 11 attribute/user-editor regression
  tests, and production build passed.
- Executed migrations 018 and 244 in an isolated PostgreSQL WASM runtime
  (PGlite): creation and repeat execution passed, no user columns or default
  grants were added, and conflicting definitions were rejected. This is a
  targeted SQL check, not a substitute for Docker-backed repository tests.

Production deployment and user permission changes are outside this source
submission. No live model requests or production database modifications were
used for validation.
