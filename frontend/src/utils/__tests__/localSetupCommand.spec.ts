import { describe, expect, it } from 'vitest'
import { buildLocalSetupCommand, resolveLocalSetupEndpoint } from '@/utils/localSetupCommand'

describe('local setup command', () => {
  const input = {
    apiKey: 'sk-test',
    baseUrl: 'https://api.example.com/v1',
    client: 'opencode' as const,
    opencodeModels: undefined,
    origin: 'https://console.example.com',
    platform: 'openai' as const
  }

  it('builds a POSIX curl-pipe-bash command with positional setup values', () => {
    const command = buildLocalSetupCommand({ ...input, os: 'unix' })
    expect(command).toContain("bash -s -- 'https://api.example.com' 'sk-test' 'opencode' 'openai' --yes")
    expect(command).toContain('sk-test')
    expect(command).toContain('https://console.example.com/scripts/sub2api-local-setup.sh')
    expect(command).not.toContain("--proto '=https'")
    expect(command).not.toContain('--tlsv1.2')
    expect(command).not.toContain('SUB2API_SETUP_PY_URL=')
    expect(command).not.toContain('SUB2API_SETUP_ENDPOINT=')
    expect(command).not.toContain('SUB2API_SETUP_API_KEY=')
    expect(command).not.toContain(' -o ')
    expect(command).not.toContain('--api-key')
  })

  it('builds a short PowerShell installer command with quote-safe positional values', () => {
    const command = buildLocalSetupCommand({ ...input, apiKey: "sk-o'hare", os: 'windows' })
    expect(command).toBe(
      "& ([scriptblock]::Create((irm 'https://console.example.com/scripts/sub2api-local-setup.ps1'))) 'https://api.example.com' 'sk-o''hare' 'opencode' 'openai' --yes"
    )
    expect(command.length).toBeLessThan(300)
    expect(command).not.toContain('powershell.exe')
    expect(command).not.toContain('SUB2API_SETUP_')
    expect(command).not.toContain('Invoke-WebRequest')
    expect(command).not.toContain('Remove-Item')
  })

  it('passes Codex WebSocket mode and OpenCode model selections positionally on Windows', () => {
    const websocket = buildLocalSetupCommand({
      ...input,
      client: 'codex',
      codexWebsocket: true,
      platform: 'openai',
      os: 'windows'
    })
    expect(websocket).toContain("'codex-ws' --yes")
    expect(websocket).not.toContain("'openai' --yes")

    const opencode = buildLocalSetupCommand({
      ...input,
      opencodeModels: ['gpt-5.5', 'gpt-5.6'],
      os: 'windows'
    })
    expect(opencode).toContain("'opencode' 'openai' --models 'gpt-5.5,gpt-5.6' --yes")
  })

  it('keeps the common OpenAI Codex command free of optional mode flags', () => {
    const command = buildLocalSetupCommand({
      ...input,
      client: 'codex',
      opencodeModels: undefined,
      os: 'unix'
    })
    expect(command).not.toContain('SUB2API_SETUP_PLATFORM=')
    expect(command).not.toContain('SUB2API_SETUP_CODEX_AUTH_MODE=')
    expect(command).not.toContain('SUB2API_SETUP_CODEX_WEBSOCKET=')
  })

  it('normalizes client endpoints consistently with the manual setup config', () => {
    expect(resolveLocalSetupEndpoint({ ...input, client: 'codex', os: 'unix' })).toBe('https://api.example.com/v1')
    expect(resolveLocalSetupEndpoint({ ...input, client: 'claude', baseUrl: 'https://api.example.com/v1', os: 'unix' })).toBe('https://api.example.com')
    expect(resolveLocalSetupEndpoint({ ...input, client: 'opencode', platform: 'gemini', baseUrl: 'https://api.example.com', os: 'unix' })).toBe('https://api.example.com/v1beta')
  })
})
