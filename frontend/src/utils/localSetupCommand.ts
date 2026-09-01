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

function commonEnvironment(input: LocalSetupCommandInput): Record<string, string> {
  const values: Record<string, string> = {
    SUB2API_SETUP_ENDPOINT: resolveSetupBaseUrl(input.baseUrl),
    SUB2API_SETUP_API_KEY: input.apiKey
  }
  if (input.client === 'opencode') {
    if (input.opencodeModels?.length) {
      values.SUB2API_SETUP_OPENCODE_MODELS = input.opencodeModels.join(',')
    }
  }
  return values
}

export function buildLocalSetupCommand(input: LocalSetupCommandInput): string {
  const origin = normalizeOrigin(input.origin)
  const scriptUrl = `${origin}/scripts/sub2api-local-setup.${input.os === 'windows' ? 'ps1' : 'sh'}`
  const assignments = Object.entries(commonEnvironment(input))

  if (input.os === 'unix') {
    const clientArg = input.client === 'codex' && input.codexWebsocket ? 'codex-ws' : input.client
    const platformArg = input.platform && !((input.client === 'codex' && input.platform === 'openai') || (input.client !== 'codex' && input.platform === 'anthropic'))
      ? ` ${shellQuote(input.platform)}`
      : ''
    const env = assignments.map(([key, value]) => `${key}=${shellQuote(value)}`).join(' ')
    const pipeline = `curl -fsSL --proto '=https' --tlsv1.2 ${shellQuote(scriptUrl)} | ${env} sh -s -- ${shellQuote(clientArg)}${platformArg} --yes`
    return `bash -o pipefail -c ${shellQuote(pipeline)}`
  }

  const env = assignments
    .map(([key, value]) => `$env:${key}=${powerShellQuote(value)}`)
    .join('; ')
  const clientArg = input.client === 'codex' && input.codexWebsocket ? 'codex-ws' : input.client
  const platformArg = input.platform && !((input.client === 'codex' && input.platform === 'openai') || (input.client !== 'codex' && input.platform === 'anthropic'))
    ? ` ${powerShellQuote(input.platform)}`
    : ''
  const script = [
    '$ErrorActionPreference = "Stop"',
    '$d = Join-Path ([IO.Path]::GetTempPath()) ("sub2api-command-" + [guid]::NewGuid().ToString("N"))',
    'New-Item -ItemType Directory -Force -Path $d | Out-Null',
    'try {',
    `  Invoke-WebRequest -UseBasicParsing -Uri ${powerShellQuote(scriptUrl)} -OutFile (Join-Path $d "setup.ps1")`,
    `  & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $d "setup.ps1") ${powerShellQuote(clientArg)}${platformArg} --yes`,
    '  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }',
    '} finally { Remove-Item -LiteralPath $d -Recurse -Force -ErrorAction SilentlyContinue }'
  ].join(' ')
  return `& { ${env}; ${script} }`
}

export function resolveLocalSetupEndpoint(input: LocalSetupCommandInput): string {
  return resolveEndpoint(input)
}
