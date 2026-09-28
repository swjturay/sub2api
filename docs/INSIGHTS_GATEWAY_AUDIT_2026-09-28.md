# Gateway dashboard audit, 2026-09-28

## Scope and safety

The initial read-only investigation covered the September 25-26 success-rate drop, lifecycle coverage and
legacy `composite` identities. Production queries used read-only transactions,
20-second statement timeouts and aggregate output. No prompts, credentials,
request bodies or user identities were exported. No production data, settings,
deployments or historical outcomes were changed during that investigation.
The subsequently authorized production repair is recorded separately below.

Dates below use Asia/Shanghai. The source under investigation is `2ae34905f`.

## Success-rate evidence

| Date | Total calls | Recorded success | Recorded failure | Stream interrupted | Rejected | Cancelled | Upstream error |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 2026-09-25 | 15,446 | 6,873 | 8,573 | 8,120 | 415 | 29 | 9 |
| 2026-09-26 | 30,981 | 9,621 | 21,360 | 19,520 | 1,760 | 78 | 2 |

The recorded success rates are 44.50% and 31.05%. Within the interrupted
population, 8,058 and 19,456 records have all of these characteristics:

- Default HTTP-sync transport and empty model/provider identity.
- A usage row linked by request ID, user ID and API key ID.
- Inbound endpoint `/v1/chat/completions`.

Of these records, 7,420 on September 25 and 19,418 on September 26 are streaming
`deepseek-flash` usage with positive output tokens. Increased traffic through
this affected path explains why the measurement defect became much more visible.

`OpenAIGatewayHandler.ChatCompletions` did not populate Insights model/provider/
transport identity and did not finalize successful forwarding. The deferred
middleware classified a response with bytes and HTTP status below 400 as
`stream_interrupted` when an upstream attempt was observed. A real handler
regression reproduced a completed response being classified as failure before
the fix, then passed after explicit success finalization was added.

This establishes a measurement defect, not a confirmed provider outage. It does
not prove that every linked usage record was successful: this handler also
records partial usage on forwarding errors. Historical repair must require
independent terminal evidence, a reviewed candidate set and reconciliation of
call facts, error facts and daily rollups. Do not blindly turn usage-linked
failures into successes or publish an invented corrected success rate. The
remaining rejection/cancellation/error counts need separate evidence to explain
their causes; their generic stored classifications do not establish a specific
quota, network or provider incident.

### Historical repair feasibility follow-up

A subsequent read-only check found retained `ops_system_logs` covering the
incident dates, but the relevant handler/access components contain warnings,
not explicit successful-terminal or successful HTTP access events. No
`openai_chat_completions.request_completed` evidence was found for September
23-28. The current backend pod was created on September 28, uses an `emptyDir`
data volume, and has no files matching `/app/data/**/*.log*`. These checks do
not rule out separate external logging systems; none was established here.

Of September 25's 8,058 candidates, three link to explicit
`openai_chat_completions.forward_failed` warnings using user/request identity
and a bounded timestamp window. September 26's 19,456 candidates have no such
matched warning, but absence of an error log is not proof of success. Therefore
the entire candidate set cannot safely be promoted to success. A possible
separate remediation is an auditable unknown-outcome classification excluded
from success-rate denominators, retaining positively identified failures; this
requires a confirmed product contract and consistent raw/daily/API treatment.
No repair or reclassification was applied during this feasibility check.

### Executed historical repair

Following explicit user confirmation, the historical policy is: retain explicit
failures; classify the remaining missing-hook candidates as success. This is an
approved historical assumption, not reconstructed evidence of successful completion.
It supersedes the conservative recommendation above for this bounded repair only.

On September 28, production repair committed successfully after a rolled-back
rehearsal. The fixed cutoff was `2026-09-28 13:35:27.598476+08:00`.
Candidates were interrupted calls with default HTTP-sync transport, empty model,
and an identity-matched Chat Completions usage record. Other failure classes and
correctly instrumented interrupted streams were not reclassified.

- Candidates: 45,878 across September 23-28; 45,873 changed to success.
- Five candidates with explicit failure evidence were retained. Evidence used
  identity- and time-bounded handler warnings, HTTP failure access logs or
  request-phase error logs. Other pre-existing failures were unchanged.
- Removed 45,873 corresponding stale error facts and rebuilt call-only daily
  aggregates atomically under the existing rollup advisory lock.
- Request totals, usage, tokens, billing and costs were unchanged. Missing model
  identity and timing were not invented.
- Backups remain in database schema `insights_repair_20260928` and private ops
  file `/root/insights-audit-20260928/history-repair-backup.dump` (6,112,090 bytes).
  A guarded rollback script is in the same directory.

| Date | Previous success rate | Repaired success rate | Success / total |
| --- | ---: | ---: | ---: |
| 2026-09-25 | 44.50% | 96.65% | 14,928 / 15,446 |
| 2026-09-26 | 31.05% | 93.85% | 29,077 / 30,981 |

Post-commit checks found no stale target errors or historical raw/daily count
mismatches. The authenticated production gateway API confirmed corrected daily
counts and rates; the public health endpoint returned `ok`. No service restart
or deployment was performed. The new handler hooks are still local pending
deployment, so requests after the cutoff can still encounter the old defect.

## Lifecycle coverage

At inspection, all 119 current users have both lifecycle coverage flags set.
105 have observed-call evidence; 14 have no observed call. The old query counted
every NULL `first_observed_call_at` as an unknown, regardless of its coverage
flag. This caused the displayed partial-coverage warning.

The query now distinguishes covered inactivity from missing history and applies
the selected cutoff when checking absence of evidence. Genuine untrusted first
anchors or return coverage still remain partial/unknown. Seven- and thirty-day
milestones may remain pending because the trusted anchors in this snapshot start
on September 23; pending is not churn or missing collection. This audit does not
certify that all pre-instrumentation lifetime history can be reconstructed.

## Model identity

Historical call facts retain the composite routing group as their platform.
The preference query previously concatenated that field directly with the model.
Raw queries now reuse the provider policy and recover a concrete account provider
from an unambiguous usage match scoped by request, user, API key and requested
model. Missing models can be recovered from that same usage row. Counts are not
multiplied. Ambiguous matches remain `unknown`; the daily-only path never exposes
`composite` as a provider. No model-name prefix heuristic is used.

The frontend resolves historical routing/unknown identities against a unique
catalog identity, combines duplicate model series and recalculates their shares
without changing department totals. Ambiguous catalog identities remain unknown.

## UI and verification

- Removed the unclassified-user card, not the backend missing-data distinction.
- Moved gateway/panel explanations into keyboard-accessible help tooltips.
- Preserved meaningful coverage warnings with less misleading lifecycle text.
- Insights lint, 84 frontend tests, TypeScript and production build passed.
- Handler regressions cover sync Responses bridge, streaming bridge, raw Chat
  success and upstream failure; the original success regression failed before
  the fix.
- Insights, migration, handler and route unit tests and `go build ./...` passed.
- Real PostgreSQL 18 integration tests passed, including cost, query, lifecycle,
  provider recovery, model, retention and late-write rollup cases. Tests ran in a
  temporary network-isolated container with bounded memory/CPU, no production
  volumes and synthetic fixtures. The container was stopped and removed.
- Local browser verification confirmed the five user metrics, keyboard tooltip
  access and merged model legend. Local Vite still proxies the production API,
  so backend code fixes are not live there. The subsequent historical data
  repair above is visible through that production API.

Code changes remain local and uncommitted. There was no publication or deployment;
the separately authorized historical data repair above was applied to production.
