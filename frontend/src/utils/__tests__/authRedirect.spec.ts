import { describe, expect, it, vi } from 'vitest'
import { isIndependentAppPath, navigateAfterAuth, sanitizeAuthRedirect } from '../authRedirect'

describe('auth redirect ownership', () => {
  it('accepts same-origin Insights deep links and rejects external redirects', () => {
    expect(sanitizeAuthRedirect('/insights/models?window=7d')).toBe('/insights/models?window=7d')
    expect(sanitizeAuthRedirect('//evil.example')).toBe('/dashboard')
    expect(sanitizeAuthRedirect('https://evil.example')).toBe('/dashboard')
    expect(isIndependentAppPath('/insights/personal')).toBe(true)
  })

  it('keeps ordinary push and replace routes in Vue Router', async () => {
    const router = { push: vi.fn(), replace: vi.fn() }
    await navigateAfterAuth(router as never, '/dashboard')
    await navigateAfterAuth(router as never, '/profile', 'replace')
    expect(router.push).toHaveBeenCalledWith('/dashboard')
    expect(router.replace).toHaveBeenCalledWith('/profile')
  })

  it('hands Insights deep links to the independent top-level app', async () => {
    const router = { push: vi.fn(), replace: vi.fn() }
    const assign = vi.fn()

    await navigateAfterAuth(router as never, '/insights/models?window=7d', 'replace', assign)

    expect(assign).toHaveBeenCalledWith('/insights/models?window=7d')
    expect(router.replace).not.toHaveBeenCalled()
  })
})
