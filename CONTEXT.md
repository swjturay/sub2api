# Usage Records

This context defines the request execution and performance terms presented in usage records.

## Language

**Request Type**:
The semantic execution category recorded for a usage event, including synchronous, streaming, WebSocket v2, live, and cyber-policy events. It is not a transport-only dimension.
_Avoid_: Client transport, upstream transport

**Client Transport**:
The protocol used by the client to call sub2api, such as synchronous HTTP, SSE, or WebSocket. It does not describe the provider-side protocol.
_Avoid_: Client request type, request type, execution type

**Upstream Transport**:
The protocol sub2api used for the provider call, such as synchronous HTTP, SSE, or WebSocket. It is independent of client transport when protocol bridging occurs.
_Avoid_: Upstream request type, endpoint, account WS mode

**Service Tier**:
The provider processing tier associated with a request, such as Standard, Fast, or Flex. It is a scheduling class, not a measured performance result.
_Avoid_: Speed mode, speed, latency tier

**Output Token Throughput**:
The average rate of output tokens produced after the first token, expressed in tokens per second. It is the reciprocal of TPOT when both use the same post-first-token interval.
_Avoid_: Generation rate, generation speed, request speed, total-request throughput

**Time per Output Token (TPOT)**:
The average time between output tokens after the first token, excluding time to first token. Lower TPOT corresponds to higher output token throughput.
_Avoid_: Time to first token, total latency, token latency

**Local Client Setup**:
A user-triggered operation that configures one Managed Agent Client in the current user's standard global configuration scope. It changes only the selected client's owned endpoint, authentication, and optional model fields; it is not client installation or account provisioning.
_Avoid_: client installation, local deployment, account setup

**Managed Configuration Field**:
A configuration value owned by the Local Client Setup operation for one client and provider. Updating it may replace the previous value, but it must not change unrelated providers or user settings.
_Avoid_: every field in the config file, global setting

**Configuration Backup**:
A timestamped copy of an existing client configuration created before a Local Client Setup update. It is a recovery artifact and remains available until the user removes it.
_Avoid_: temporary file, configuration cache
