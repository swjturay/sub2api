import type { Account } from '@/types'

export type CPRProxyAccount = Pick<Account, 'platform' | 'type' | 'extra'>

export function sanitizeCPRProxyEndpoint(raw: unknown): string {
  if (typeof raw !== 'string' || [...raw].some(c => c.charCodeAt(0) < 32 || c.charCodeAt(0) === 127)) return ''
  try {
    const url = new URL(raw.trim())
    if (!['http:', 'https:', 'socks5:', 'socks5h:'].includes(url.protocol) || !url.hostname) return ''
    const protocol = url.protocol === 'socks5:' ? 'socks5h:' : url.protocol
    return protocol + '//' + url.host
  } catch {
    return ''
  }
}

export function cprOutboundProxy(account: CPRProxyAccount | null | undefined) {
  if (account?.platform !== 'openai' || account.type !== 'cpr') return null
  const extra = account.extra ?? {}
  const endpoint = sanitizeCPRProxyEndpoint(extra.cpr_outbound_proxy)
  const declared = extra.cpr_outbound_proxy_status
  const status = declared === 'direct' && !extra.cpr_outbound_proxy
    ? 'direct'
    : endpoint && (declared === 'proxy' || declared === undefined) ? 'proxy' : 'unknown'
  const rawTime = extra.cpr_outbound_proxy_updated_at
  const observedAt = typeof rawTime === 'string' && Number.isFinite(Date.parse(rawTime)) ? rawTime : null
  return { endpoint: status === 'proxy' ? endpoint : '', status, observedAt }
}
