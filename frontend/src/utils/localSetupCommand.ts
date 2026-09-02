import type { GroupPlatform } from '@/types'

export type LocalSetupClient = 'claude' | 'codex' | 'opencode'
export type LocalSetupOS = 'unix' | 'windows'

export interface LocalSetupCommandInput {
  apiKey: string
  baseUrl: string
  client: LocalSetupClient
  codexWebsocket?: boolean
  opencodeModels?: string[]
  origin?: string
  platform?: GroupPlatform | null
  os: LocalSetupOS
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`
}

function powerShellQuote(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

function normalizeOrigin(origin?: string): string {
  const fallback = typeof window !== 'undefined' ? window.location.origin : ''
  return (origin || fallback).replace(/\/+$/, '')
}

function normalizeBaseRoot(baseUrl: string): string {
  return (baseUrl || '').trim().replace(/\/v1\/?$/i, '').replace(/\/+$/, '')
}

function ensureV1(baseUrl: string): string {
  const value = baseUrl.replace(/\/+$/, '')
  return /\/v1$/i.test(value) ? value : `${value}/v1`
}

function resolveSetupBaseUrl(baseUrl: string): string {
  return normalizeBaseRoot(baseUrl)
}

function resolveEndpoint(input: LocalSetupCommandInput): string {
  const root = normalizeBaseRoot(input.baseUrl)
  if (input.client === 'claude') {
    // Claude Code appends /v1/messages itself.
    return input.platform === 'antigravity' ? `${root}/antigravity` : root
  }
  if (input.platform === 'gemini' && input.client === 'opencode') {
    const value = root.endsWith('/v1beta') ? root : `${root}/v1beta`
    return value.replace(/\/+$/, '')
  }
  if (input.platform === 'antigravity' && input.client === 'opencode') {
    return ensureV1(`${root}/antigravity`)
  }
  return ensureV1(root)
}

export function buildLocalSetupCommand(input: LocalSetupCommandInput): string {
  const origin = normalizeOrigin(input.origin)
  const scriptUrl = `${origin}/scripts/sub2api-local-setup.${input.os === 'windows' ? 'ps1' : 'sh'}`
  const endpointValue = resolveSetupBaseUrl(input.baseUrl)
  const endpointArg = input.os === 'windows' ? powerShellQuote(endpointValue) : shellQuote(endpointValue)
  const apiKeyArg = input.os === 'windows' ? powerShellQuote(input.apiKey) : shellQuote(input.apiKey)
  const clientArg = input.client === 'codex' && input.codexWebsocket ? 'codex-ws' : input.client
  const platformArg = input.platform && !((input.client === 'codex' && input.platform === 'openai') || (input.client !== 'codex' && input.platform === 'anthropic'))
    ? ` ${shellQuote(input.platform)}`
    : ''
  const modelArg = input.opencodeModels?.length
    ? ` --models ${shellQuote(input.opencodeModels.join(','))}`
    : ''

  if (input.os === 'unix') {
    return `curl -fsSL ${shellQuote(scriptUrl)} | bash -s -- ${endpointArg} ${apiKeyArg} ${shellQuote(clientArg)}${platformArg}${modelArg} --yes`
  }

  const assignments = [
    `$env:SUB2API_SETUP_ENDPOINT=${endpointArg}`,
    `$env:SUB2API_SETUP_API_KEY=${apiKeyArg}`,
    ...(input.client === 'codex' ? [] : [`$env:SUB2API_SETUP_CLIENT=${powerShellQuote(clientArg)}`]),
    ...(input.client === 'codex' && input.codexWebsocket
      ? ['$env:SUB2API_SETUP_CODEX_WEBSOCKET=\'true\'']
      : []),
    ...(input.platform && !((input.client === 'codex' && input.platform === 'openai') || (input.client !== 'codex' && input.platform === 'anthropic'))
      ? [`$env:SUB2API_SETUP_PLATFORM=${powerShellQuote(input.platform)}`]
      : []),
    ...(input.opencodeModels?.length
      ? [`$env:SUB2API_SETUP_OPENCODE_MODELS=${powerShellQuote(input.opencodeModels.join(','))}`]
      : []),
  ].join('; ')
  return `& { ${assignments}; irm ${powerShellQuote(scriptUrl)} | iex }`
}

export function resolveLocalSetupEndpoint(input: LocalSetupCommandInput): string {
  return resolveEndpoint(input)
}
