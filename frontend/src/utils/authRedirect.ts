import type { Router } from 'vue-router'

export function sanitizeAuthRedirect(raw: unknown, fallback = '/dashboard'): string {
  if (typeof raw !== 'string' || !raw.startsWith('/') || raw.startsWith('//')) return fallback
  if (raw.includes('://') || raw.includes('\n') || raw.includes('\r')) return fallback
  return raw
}

export function isIndependentAppPath(path: string): boolean {
  return path === '/insights' || path.startsWith('/insights/') || path.startsWith('/insights?')
}

export async function navigateAfterAuth(
  router: Router,
  rawRedirect: unknown,
  mode: 'push' | 'replace' = 'push',
  assign: (path: string) => void = path => window.location.assign(path),
): Promise<void> {
  const redirect = sanitizeAuthRedirect(rawRedirect)
  if (isIndependentAppPath(redirect)) {
    assign(redirect)
    return
  }
  await router[mode](redirect)
}
