import { describe, expect, it } from 'vitest'
import { cprOutboundProxy, sanitizeCPRProxyEndpoint, type CPRProxyAccount } from '../cprOutboundProxy'

const account = (extra: Record<string, unknown>): CPRProxyAccount => ({ platform: 'openai', type: 'cpr', extra })

describe('CPR outbound proxy display', () => {
  it.each([
    ['socks5h://user:p@ss@host:1080/path?secret=value#secret', 'socks5h://host:1080'],
    ['http://user:pass@host:8080/%73ecret?token=secret', 'http://host:8080'],
    ['socks5://user:pass@[::1]:1080', 'socks5h://[::1]:1080'],
    ['https://host:8443', 'https://host:8443'],
    ['javascript:alert(1)', ''],
    ['ftp://user:pass@host', ''],
    ['not a URL', ''],
    ['http://host:bad', ''],
    ['http://host\n/secret', '']
  ])('sanitizes %s', (input, expected) => {
    expect(sanitizeCPRProxyEndpoint(input)).toBe(expected)
  })

  it('distinguishes direct, unknown, invalid, and legacy snapshots', () => {
    expect(cprOutboundProxy(account({}))?.status).toBe('unknown')
    expect(cprOutboundProxy(account({ cpr_outbound_proxy_status: 'direct', cpr_outbound_proxy: '' }))?.status).toBe('direct')
    expect(cprOutboundProxy(account({ cpr_outbound_proxy_status: 'proxy', cpr_outbound_proxy: 'invalid' }))?.status).toBe('unknown')
    expect(cprOutboundProxy(account({ cpr_outbound_proxy: 'socks5h://user:secret@host:1080' }))?.endpoint).toBe('socks5h://host:1080')
    expect(cprOutboundProxy(account({ cpr_outbound_proxy_status: 'unknown', cpr_outbound_proxy: 'http://old:8080' }))?.endpoint).toBe('')
  })

  it('does not expose stale CPR metadata on another account type', () => {
    expect(cprOutboundProxy({ ...account({ cpr_outbound_proxy: 'http://old:8080' }), type: 'oauth' })).toBeNull()
    expect(cprOutboundProxy(null)).toBeNull()
  })

  it('only displays a valid observation time', () => {
    expect(cprOutboundProxy(account({ cpr_outbound_proxy_updated_at: 'invalid' }))?.observedAt).toBeNull()
    expect(cprOutboundProxy(account({ cpr_outbound_proxy_updated_at: '2026-09-21T00:00:00Z' }))?.observedAt).toBe('2026-09-21T00:00:00Z')
  })
})
