# CPR credential health repair

Baseline: production 899ec923a6584902d935cea82d9c78aa44d43bb8.

## Accepted behavior

The owner requested completion of the CPR repair after diagnosing repeated
503 no_available_provider responses from a binding whose CPR OAuth credential
had been revoked. The local binding remained active and schedulable.

- A received structured availability/credential error triggers a bounded
  management check of the configured CPR account, never an OpenAI probe.
- Confirmed credential failure or explicit upstream disablement quarantines
  the binding using existing error/scheduling fields and a sanitized reason.
- Temporary overload, model rejection and quota are not permanent auth failures.
  Management failure can cause only a short runtime cooldown, not quarantine.
- HTTP, SSE, converted Chat/Messages, WS frames/handshakes and uncaptured
  endpoints preserve payloads, measured usage, cancellation and no replay after
  output. A failed credential attempt does not retry the same binding.
- Writes guard credentials, row version and manual state. Scheduler outbox and
  quarantine are atomic; local recovery and runtime blocking share a lock.
- Concurrent checks coalesce. Health failures are not cached across checks.
- A lifecycle-owned minute scan reads management for active schedulable CPR
  bindings. It never restores paused accounts or sends inference probes.
- Reauthorization and successful verification precede explicit manual recovery.

## Validation

The initial real HTTP relay regression failed because the confirmed invalid
binding remained schedulable; it passes after the shared health observer was
added. The WS regression independently failed before its observer was connected.
Further protocol, concurrency and database verification is recorded with the
final source/release validation; this document is not a claim of deployment.

Focused CPR/native relay tests and credential-health race tests pass. The
initial HTTP, actual scheduler-predicate and second-WS-turn regressions each
failed before their respective fixes. Standards review found two issues and
spec review found two; all were fixed and both reviewers reported no remaining
blocking findings. golangci-lint 2.14.0 reports zero issues.

Real PostgreSQL integration coverage checks quarantine exclusion, conditional
writes and the durable scheduler event. Local execution skips because Docker is
unavailable; the release requires the Linux CI integration job to pass.
