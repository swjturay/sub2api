import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import CPROutboundProxy from '../CPROutboundProxy.vue'
import type { CPRProxyAccount } from '@/utils/cprOutboundProxy'

enableAutoUnmount(afterEach)
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const account = (extra: Record<string, unknown>): CPRProxyAccount => ({ platform: 'openai', type: 'cpr', extra })

describe('CPROutboundProxy', () => {
  it('renders redacted endpoints and clears them when the account becomes direct', async () => {
    const wrapper = mount(CPROutboundProxy, { props: { account: account({
      cpr_outbound_proxy: 'socks5h://user:p@ss@host:1080/path?token=secret',
      cpr_outbound_proxy_status: 'proxy',
      cpr_outbound_proxy_updated_at: '2026-09-21T00:00:00Z'
    }) } })
    expect(wrapper.text()).toContain('socks5h://host:1080')
    expect(wrapper.html()).not.toContain('secret')
    expect(wrapper.html()).not.toContain('p@ss')
    expect(wrapper.find('time').attributes('datetime')).toBe('2026-09-21T00:00:00Z')
    await wrapper.setProps({ account: account({ cpr_outbound_proxy: '', cpr_outbound_proxy_status: 'direct' }) })
    expect(wrapper.text()).toContain('admin.accounts.cprOutboundDirect')
    expect(wrapper.text()).not.toContain('host:1080')
    await wrapper.setProps({ account: account({}) })
    expect(wrapper.text()).toContain('admin.accounts.cprOutboundUnknown')
    expect(wrapper.find('time').exists()).toBe(false)
  })

  it('does not render for non-CPR accounts', () => {
    const wrapper = mount(CPROutboundProxy, { props: { account: { ...account({}), type: 'oauth' } } })
    expect(wrapper.find('[data-testid="account-cpr-outbound"]').exists()).toBe(false)
  })
})
