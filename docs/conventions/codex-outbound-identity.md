# Codex Outbound Identity

## Direct OAuth And Setup Tokens

Normalize User-Agent and version after copying client headers and applying the
account's configured identity. Missing originator means preserve its absence;
it must not bypass UA normalization. ForceCodexCLI takes precedence over an
account UA on inference, alpha search, and token-counting requests.

Regression tests must call production request builders and inspect what an
HTTP test server receives, including compatibility bridges and setup tokens.

OAuth/setup-token Responses input_tokens and Anthropic count_tokens requests
estimate locally after model mapping/conversion, before credential retrieval,
refresh or upstream dispatch. Reuse the existing estimator and minimum fallback;
these are approximate preflight counts, never billing usage. Logs identify the
direct estimate with reason=oauth_local and upstream_status=0. Preserve API-key
counting policy. CPR also estimates both count endpoints locally (cpr_local for
Messages), since its inference server exposes Responses rather than Messages.

Token import must stop if an explicitly selected proxy cannot be resolved,
including an empty lookup result. Token exchange and refresh reuse connections
but disable the shared cookie jar; cookie policy participates in the pool key.

Native Lite requests with the device wire profile keep their existing developer
instructions without synthesizing a second top-level prompt. This exception
requires an unchanged model and the native additional-tools shape. Compact
fallback restores the omitted default prompt when switching models, without
overwriting supplied instructions. Modifying an already compressed Lite body
must recompress it and update its length and replay body together.

Native device-profile requests preserve upstream-shaped `call_*` IDs across
calls, outputs and references. Keep legacy/third-party normalization separate.
After model mapping, align existing turn-metadata model fields with the final
HTTP body or WS frame. Preserve missing/invalid metadata, unknown fields and
the user's reasoning-effort selection. Lite compatibility must edit the
finalized request body so a repeated bridge pass cannot undo model alignment.

## Passive Codex Observations

Capture HTTP observations per attempt at transport dispatch/response, and carry
the immutable snapshot into usage recording. Never reread shared cookie state.
Safety header presence is independent of its parsed Boolean value. The retry
model hint is not evidence of the model serving the request. Preserve those
headers on HTTP Responses and compatibility-bridge responses, subject to the
existing first-output/keepalive response-commit boundary.

Persist only account-scoped HMAC-SHA256 route digests, with separate outbound
and response observations. The configured JWT secret is domain-separated for
this purpose; changing it resets digest correlation, and missing configuration
disables digests. A bounded, unverified response gateway hint is diagnostic
only. Never persist raw cookies or use observations for billing/scheduling.

Only administrators receive `codex_observation`. Historical/unobserved rows
remain NULL. Direct native WS turns report `websocket_unobserved`; pooled
handshake headers are not per-turn evidence. CPR WS can additionally carry
received per-turn usage and safety metadata; this does not imply route visibility. A WS-to-HTTP bridge reports its
actual HTTP observation. This feature introduces no Cookie storage or replay,
so an absent outbound route digest is expected when no cookie was sent.

## CPR And API-Key Destinations

CPR owns the external OpenAI transport and identity. Do not classify CPR as a
direct Codex-protocol account or inject direct OAuth identity headers into it.
Model discovery identifies the gateway as sub2api; API-key header overrides
retain precedence. The internal CPR hop does not determine public egress.

CPR quota snapshots come only from the CPR Admin API. Sub2API still owns quota,
scheduling, model, and rate-limit policy, but must not query OpenAI directly on
behalf of a CPR account or silently fall back to a direct OAuth path.
An admin refresh failure must expose a sanitized error, retain the last valid
quota and its observation time, and leave missing windows unknown. Local usage
statistics cannot create a zero-percent CPR quota. Proxy/plan observations do
not make an unavailable quota fresh. Keep the refresh action available beside
the error and any retained quota bars.

## CPR Native Relay And Accounting

Only CPR uses the native HTTP/SSE and WebSocket raw relay. Preserve original
wire bytes (including request compression), unknown fields and call IDs when
no authorized group/account policy changes them. Retain model mappings,
reasoning and Fast policy, every-turn image permission, and passive turn-state
provenance checks. Do not add API-key raw switches or direct OAuth rewrites.
Chat/Messages still convert to/from Responses, without truncating CPR history
or adding the non-Codex todo guard. These paths retain existing mapping rules.

All CPR inference paths cancel upstream on client disconnect. Once execution
starts, user and account slots remain owned until local upstream teardown has
returned; waiting cancellation still releases promptly. WS applies the same
rule per turn. There is no background drain to obtain the final bill.

Capture bounded received usage before downstream writes/conversion. Valid
numeric zero differs from missing/empty/malformed usage; terminal usage with
both counters is complete, progressive or one-counter usage is partial, and
no evidence is unknown. Persist completeness in admin-only codex_observation;
this diagnostic field is never pricing input. Bill the separate captured real
counters, including failed/cancelled and Cyber results. Never bill token-count
estimates as generation usage. A CPR attempt/turn has an independent billing
ID so failover attempts cannot suppress one another under one client ID;
resubmitting one captured result uses the same billing ID. Cyber alerts remain
active, with a single authoritative captured result for settlement.

Use the existing failover counts and KLNO raw-relay error classification.
CPR does not inherit the direct OAuth 120-second retry window, deferred 429
cooldown or extra account-switch cap. A structured account-quota error stops
same-account retry, including pool mode. Specific model/key/queue/capacity codes
are not account-quota evidence; do not classify free-form messages.

Observe CPR quota failures beside the received usage. Preserve valid reset
evidence from the response or the same WS error frame. Without it, query the
bound CPR management account with a two-second timeout and per-binding
singleflight/short caching. Only confirmed quota_exhausted main windows or
rate_limited account state supply a future next-attempt time; aggregate display
limitReached and fetch timestamps are not authority or fresh quota evidence.
Use the existing configured fallback when no future time is available. Extend
the existing rate-limit field atomically with SetRateLimitedIfLater and notify
the existing runtime blocker; generic concurrent CPR 429s must not shorten it.
Do not add disabled/error state, recovery timers, probes or database fields.
Streaming quota updates run after writing the received failure frame; upstream
teardown still cancels immediately. The management query can delay local
settlement/release by up to its timeout. A reset is permission to try again,
not proof that CPR has recovered. Every attempt/turn settles this fact once.

Do not replay after generated output or cancellation. Retry before response
can still duplicate remote execution; incomplete usage is not reconciled
or estimated automatically. HTTP CPR observations contain safety/completeness
but no route digest of the internal hop. No new Cookie replay is introduced.

## Codex Management-Plane Clients

Keep the Codex backend and ChatGPT browser transports separate. WHAM usage,
rate-limit reset credits, spendable credits, and quota-adjacent exit probing use
`CodexBackendClientFactory`: normal HTTP transport through the account proxy,
canonical Codex identity, and no browser impersonation.

Privacy settings, ChatGPT account/subscription discovery, and referrals use
`PrivacyClientFactory` or a business client built from it. Those calls retain
the browser-shaped transport needed by ChatGPT Web endpoints. Never silently
fall back from one factory to the other; a missing dependency is a configuration
error so transport identity cannot drift unnoticed.

## CPR Outbound Display

Only display CPR observations on OpenAI CPR accounts. Persist a sanitized proxy
endpoint, a proxy/direct/unknown status, and the last observation time. Rebuild
endpoints from scheme and authority only; never log raw parse errors or retain
userinfo, paths, queries, or fragments.

An explicit null or empty endpoint is direct. Missing, invalid, or unsupported
endpoint data is unknown. Both clear an old displayed proxy. Failed admin
requests retain the previous observation and its timestamp. Unchanged display
observations are written at most every five minutes; route changes apply
immediately. These fields are scheduler-neutral and cannot freshen quota data.

The endpoint is configured routing information, not a measured public IP.
Passive cross-account turn-state guards remain enabled; this intake adds no
turn-state injection, pooling, hunter, or recovery probes.

## Frontend Checks

Account expiry and countdown displays use the scope-owned shared 30-second clock.
The last consumer releases its timer. Clock updates never mutate account state.
Compile every locale message in the build and critical CI suite. Valid plural
alternatives without placeholders remain supported.
