# Adopt selected klno improvements

Date: 2026-10-30 (Asia/Tokyo). Scope: cce-deploy source changes only.

## Included

- OAuth non-stream passthrough detects SSE framing even when Content-Type is
  absent and preserves the caller's stream=false response shape.
- CPR Admin API discovery lists OpenAI accounts with bounded pagination and
  sanitized errors. Save-time setup reuses or creates a single-account group
  and a group-scoped Client Key under an in-process idempotency lock.
- CPR admin keys are never copied to a changed destination; the old key can be
  reused only when the admin URL is unchanged. The UI can discover and select an
  account, while manual Client Key input remains compatible.
- CI workflows cancel superseded branch runs. The upstream sync workflow is
  non-publishing by default and requires explicit publish input.

## Excluded

Gateway Pool, turn-state quality recovery, raw upstream recording, and klno
migrations are intentionally excluded. Their schemas and runtime boundaries do
not match cce-deploy.

## Validation

- OAuth no-Content-Type SSE regression passed.
- CPR connection and existing-admin-key destination tests passed.
- Backend unit compile across all packages passed.
- Frontend CreateAccountModal tests (49), typecheck and ESLint passed.
- Workflow YAML parsed locally; actionlint was unavailable on this host.

No image was built and no production deployment was performed.
