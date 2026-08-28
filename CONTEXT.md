# Usage Records

This context defines the request execution and performance terms presented in usage records.

## Language

**Client Request Type**:
The transport style used by the client when calling sub2api, such as synchronous HTTP, SSE, or WebSocket. It does not describe the transport selected for the upstream account.
_Avoid_: Request type, upstream request type, execution type

**Upstream Request Type**:
The transport style sub2api used for the provider request, such as synchronous HTTP, SSE, or WebSocket. It is independent of the client request type when protocol bridging occurs.
_Avoid_: Client request type, endpoint, account WS mode

**Speed Mode**:
The provider execution tier actually associated with a request, such as Standard, Fast, or Flex. It is a selected service tier, not a measured performance result.
_Avoid_: Speed, generation speed, latency tier

**Generation Rate**:
The average number of output tokens produced per second after the first token, excluding time-to-first-token from the measurement interval.
_Avoid_: Speed mode, request speed, total-request throughput
