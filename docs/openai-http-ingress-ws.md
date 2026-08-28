# OpenAI Responses HTTP/SSE ingress to upstream WebSocket

This change keeps the client-facing `POST /v1/responses` contract unchanged
while allowing an eligible OpenAI account to execute a streaming turn through
the existing Responses WebSocket v2 connection pool.

## Enablement

The bridge is opt-in at two levels:

```yaml
gateway:
  openai_ws:
    http_ingress_enabled: true
```

The selected account must resolve to `ctx_pool`. `passthrough` remains the
native downstream WebSocket relay mode, and `http_bridge` retains its existing
WS-to-HTTP meaning. The bridge currently applies only to streaming
`/v1/responses` requests; compact paths and other protocols remain on their
existing transports.

## Wire behavior

Sub2API normalizes and authorizes the HTTP request, builds an upstream
`response.create` payload, acquires a pooled upstream WebSocket, and relays
each valid Responses event as an SSE `data:` frame. Usage and terminal-event
handling remain in the existing `forwardOpenAIWSV2` implementation.

The HTTP handler permits OAuth continuation with `previous_response_id` only
when the selected account is actually eligible for this bridge. API-key HTTP
continuation behavior is unchanged.

## Safety boundary

Before downstream semantic output is written, an exhausted WS attempt may
re-enter the HTTP adapter once. The feature does not replay a request after
downstream semantic output has been written; existing post-output failure rules
remain authoritative.
`/v1/chat/completions`, `/v1/messages`, `/v1/responses/compact`, and native
downstream WebSocket ingress are outside this first-stage bridge.

## Validation

The following checks pass on the `swjturay/sub2api` `release` baseline:

```text
go test ./internal/config -count=1
go test ./internal/service -run '^(TestOpenAIGatewayService_Forward_HTTPIngress|TestOpenAIHTTPIngressWSBridgeRequiresCtxPoolAndFlag|TestForwardOpenAIWSV2_)' -count=1
go test ./internal/handler -run 'TestOpenAIResponses_(AcceptsHTTPContinuationPreviousResponseIDBeforeRouting|RejectsUnownedHTTPContinuation|FunctionCallOutputHTTPGuidanceDoesNotSuggestPreviousResponseReuse)|TestOpenAIResponsesWebSocket_' -count=1
go vet ./internal/config ./internal/service ./internal/handler
```

The new end-to-end-shaped service test verifies that an HTTP request marked
`stream=true` acquires the WS pool, emits `text/event-stream`, relays upstream
delta/completed events, and records the WS transport decision. The complete
`internal/service` suite also contains unrelated pre-existing failures in
content-moderation/plugin-environment tests; those failures are outside this
change and do not occur in the focused bridge tests above.
