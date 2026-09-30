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
counting policy and CPR's separate native/Anthropic behavior.

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
remain NULL. Native WS turns explicitly report `websocket_unobserved`; pooled
handshake headers are not per-turn evidence. A WS-to-HTTP bridge reports its
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
