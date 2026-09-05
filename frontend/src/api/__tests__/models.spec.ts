import { describe, expect, it, vi } from 'vitest'
import { fetchGatewayModels } from '@/api/models'

describe('fetchGatewayModels', () => {
  it('normalizes OpenAI and Gemini model response shapes', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ data: [{ id: 'gpt-test' }, { id: 'gpt-test' }] }) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ models: [{ name: 'models/gemini-test' }] }) }))

    await expect(fetchGatewayModels('https://api.example.com/v1', 'sk-test')).resolves.toEqual([{ id: 'gpt-test' }])
    await expect(fetchGatewayModels('https://api.example.com', 'sk-test')).resolves.toEqual([{ id: 'gemini-test' }])
    expect(fetch).toHaveBeenNthCalledWith(
      2,
      'https://api.example.com/v1/models',
      expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer sk-test' }) })
    )
  })
})
