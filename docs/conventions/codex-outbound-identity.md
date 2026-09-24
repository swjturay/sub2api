# Codex Outbound Identity

## Direct OAuth And Setup Tokens

Normalize User-Agent and version after copying client headers and applying the
account's configured identity. Missing originator means preserve its absence;
it must not bypass UA normalization. ForceCodexCLI takes precedence over an
account UA on inference, alpha search, and token-counting requests.

Regression tests must call production request builders and inspect what an
HTTP test server receives, including compatibility bridges and setup tokens.

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
