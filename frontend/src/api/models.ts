export interface GatewayModelOption {
  id: string
}

function normalizeModelsEndpoint(baseUrl: string): string {
  const normalized = (baseUrl || '').trim().replace(/\/+$/, '')
  if (/\/v1(beta)?$/i.test(normalized)) return `${normalized}/models`
  return `${normalized}/v1/models`
}

function modelId(value: unknown): string | null {
  if (typeof value === 'string') return value.replace(/^models\//, '')
  if (!value || typeof value !== 'object') return null
  const item = value as { id?: unknown; name?: unknown }
  const id = typeof item.id === 'string' ? item.id : item.name
  return typeof id === 'string' ? id.replace(/^models\//, '') : null
}

export async function fetchGatewayModels(baseUrl: string, apiKey: string, signal?: AbortSignal): Promise<GatewayModelOption[]> {
  const response = await fetch(normalizeModelsEndpoint(baseUrl), {
    headers: {
      Accept: 'application/json',
      Authorization: `Bearer ${apiKey}`
    },
    cache: 'no-store',
    signal
  })
  if (!response.ok) throw new Error(`Models request failed with status ${response.status}`)

  const payload: unknown = await response.json()
  const values = payload && typeof payload === 'object'
    ? ((payload as { data?: unknown[]; models?: unknown[] }).data
      ?? (payload as { models?: unknown[] }).models
      ?? [])
    : []
  return values
    .map(modelId)
    .filter((id): id is string => Boolean(id))
    .filter((id, index, all) => all.indexOf(id) === index)
    .map((id) => ({ id }))
}
