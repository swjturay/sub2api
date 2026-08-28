import { describe, expect, it } from 'vitest'

import { resolveClientRequestType, resolveUpstreamRequestType } from '@/utils/usageRequestType'

describe('usage request transports', () => {
  it('keeps client SSE distinct from upstream WS for bridged requests', () => {
    const usage = {
      client_request_type: 'sse',
      request_type: 'ws_v2',
      stream: true,
      openai_ws_mode: true,
    }

    expect(resolveClientRequestType(usage)).toBe('sse')
    expect(resolveUpstreamRequestType(usage)).toBe('ws')
  })

  it('uses only unambiguous legacy client request type fallbacks', () => {
    expect(resolveClientRequestType({ request_type: 'sync' })).toBe('sync')
    expect(resolveClientRequestType({ request_type: 'stream', stream: true })).toBe('sse')
    expect(resolveClientRequestType({ request_type: 'ws_v2', stream: true, openai_ws_mode: true })).toBe('unknown')
  })

  it('maps native and live upstream websocket requests', () => {
    expect(resolveUpstreamRequestType({ request_type: 'ws_v2' })).toBe('ws')
    expect(resolveUpstreamRequestType({ request_type: 'live' })).toBe('ws')
  })
})
