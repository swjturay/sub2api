import type { UsageRequestType, UsageTransportType } from '@/types'

export interface UsageRequestTypeLike {
  request_type?: string | null
  client_request_type?: string | null
  stream?: boolean | null
  openai_ws_mode?: boolean | null
}

const VALID_REQUEST_TYPES = new Set<UsageRequestType>(['unknown', 'sync', 'stream', 'ws_v2', 'cyber', 'live'])
const VALID_TRANSPORT_TYPES = new Set<UsageTransportType>(['unknown', 'sync', 'sse', 'ws'])

export const isUsageRequestType = (value: unknown): value is UsageRequestType => {
  return typeof value === 'string' && VALID_REQUEST_TYPES.has(value as UsageRequestType)
}

export const isUsageTransportType = (value: unknown): value is UsageTransportType => {
  return typeof value === 'string' && VALID_TRANSPORT_TYPES.has(value as UsageTransportType)
}

export const resolveUsageRequestType = (value: UsageRequestTypeLike): UsageRequestType => {
  if (isUsageRequestType(value.request_type)) {
    return value.request_type
  }
  if (value.openai_ws_mode) {
    return 'ws_v2'
  }
  return value.stream ? 'stream' : 'sync'
}

export const resolveClientTransport = (value: UsageRequestTypeLike): UsageTransportType => {
  if (isUsageTransportType(value.client_request_type) && value.client_request_type !== 'unknown') {
    return value.client_request_type
  }
  const requestType = resolveUsageRequestType(value)
  if (requestType === 'sync') return 'sync'
  if (requestType === 'stream') return 'sse'
  return 'unknown'
}

export const resolveUpstreamTransport = (value: UsageRequestTypeLike): UsageTransportType => {
  const requestType = resolveUsageRequestType(value)
  if (requestType === 'ws_v2' || requestType === 'live' || value.openai_ws_mode) return 'ws'
  if (requestType === 'stream' || value.stream) return 'sse'
  if (requestType === 'sync' || requestType === 'cyber') return 'sync'
  return 'unknown'
}

type UsageLabelTranslator = (key: string) => string

export const formatUsageTransport = (
  transport: UsageTransportType,
  translate: UsageLabelTranslator,
  unknownLabel = '',
): string => {
  if (transport === 'sse') return 'SSE'
  if (transport === 'ws') return 'WS'
  if (transport === 'sync') return translate('usage.sync')
  return unknownLabel
}

export const formatUsageRequestType = (
  value: UsageRequestTypeLike,
  translate: UsageLabelTranslator,
): string => {
  const requestType = resolveUsageRequestType(value)
  if (requestType === 'cyber') return translate('usage.cyber')
  if (requestType === 'live') return translate('usage.live')
  if (requestType === 'ws_v2') return translate('usage.ws')
  if (requestType === 'stream') return translate('usage.stream')
  if (requestType === 'sync') return translate('usage.sync')
  return translate('usage.unknown')
}

export const requestTypeToLegacyStream = (requestType?: UsageRequestType | null): boolean | null | undefined => {
  // cyber 与 stream 正交（cyber 可发生在 stream 或非 stream 请求），不映射到 legacy stream 维度。
  if (!requestType || requestType === 'unknown' || requestType === 'cyber' || requestType === 'live') {
    return null
  }
  if (requestType === 'sync') {
    return false
  }
  return true
}
