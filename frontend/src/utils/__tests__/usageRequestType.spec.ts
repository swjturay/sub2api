import { describe, expect, it } from 'vitest'

import {
  formatUsageRequestType,
  formatUsageTransport,
  resolveClientTransport,
  resolveUpstreamTransport,
} from '@/utils/usageRequestType'

const labels: Record<string, string> = {
  'usage.cyber': 'Cyber',
  'usage.live': 'Live',
  'usage.ws': 'WS',
  'usage.stream': 'Stream',
  'usage.sync': 'Sync',
  'usage.unknown': 'Unknown',
}
const t = (key: string): string => labels[key] ?? key

describe('usage request transports', () => {
  it('keeps client SSE distinct from upstream WS for bridged requests', () => {
    const usage = {
      client_request_type: 'sse',
      request_type: 'ws_v2',
      stream: true,
      openai_ws_mode: true,
    }

    expect(resolveClientTransport(usage)).toBe('sse')
    expect(resolveUpstreamTransport(usage)).toBe('ws')
  })

  it('uses only unambiguous legacy client transport fallbacks', () => {
    expect(resolveClientTransport({ request_type: 'sync' })).toBe('sync')
    expect(resolveClientTransport({ request_type: 'stream', stream: true })).toBe('sse')
    expect(resolveClientTransport({ request_type: 'ws_v2', stream: true, openai_ws_mode: true })).toBe('unknown')
  })

  it('maps native and live upstream websocket requests', () => {
    expect(resolveUpstreamTransport({ request_type: 'ws_v2' })).toBe('ws')
    expect(resolveUpstreamTransport({ request_type: 'live' })).toBe('ws')
  })

  it('formats semantic request types without collapsing live or cyber', () => {
    expect(formatUsageRequestType({ request_type: 'live' }, t)).toBe('Live')
    expect(formatUsageRequestType({ request_type: 'cyber' }, t)).toBe('Cyber')
    expect(formatUsageRequestType({ request_type: 'stream' }, t)).toBe('Stream')
  })

  it('formats transport labels independently from request type labels', () => {
    expect(formatUsageTransport('sse', t)).toBe('SSE')
    expect(formatUsageTransport('ws', t)).toBe('WS')
    expect(formatUsageTransport('sync', t)).toBe('Sync')
    expect(formatUsageTransport('unknown', t, '-')).toBe('-')
  })
})
