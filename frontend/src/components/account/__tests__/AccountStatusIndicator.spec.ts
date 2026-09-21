import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AccountStatusIndicator from '../AccountStatusIndicator.vue'
import type { Account } from '@/types'

enableAutoUnmount(afterEach)
afterEach(() => vi.useRealTimers())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return {
    ...actual,
    formatCountdown: (_target: unknown, now?: number) => 'remaining-at-' + now,
    formatCountdownWithSuffix: (_target: unknown, now?: number) => 'remaining-at-' + now
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'antigravity',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

describe('AccountStatusIndicator', () => {
  it('updates the displayed countdown before the account expires', async () => {
    vi.useFakeTimers()
    const start = Date.parse('2026-09-21T00:00:00Z')
    vi.setSystemTime(start)
    const wrapper = mount(AccountStatusIndicator, {
      props: { account: makeAccount({ overload_until: '2026-09-21T00:05:00Z' }) },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.text()).toContain('remaining-at-' + start)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(wrapper.text()).toContain('remaining-at-' + (start + 30_000))
  })
  it.each([
    ['rate_limit_reset_at', 'admin.accounts.status.rateLimited'],
    ['overload_until', 'admin.accounts.status.overloaded'],
    ['temp_unschedulable_until', 'admin.accounts.status.tempUnschedulable']
  ] as const)('expires %s without a props refresh', async (field, label) => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-21T00:00:00Z'))
    const account = makeAccount({ [field]: '2026-09-21T00:01:00Z' })
    const wrapper = mount(AccountStatusIndicator, { props: { account }, global: { stubs: { Icon: true } } })
    expect(wrapper.text()).toContain(label)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(wrapper.text()).not.toContain(label)
    expect(account[field]).toBe('2026-09-21T00:01:00Z')
  })

  it('shares one timer across rows and releases it when the last row unmounts', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-21T00:00:00Z'))
    const props = { account: makeAccount({ extra: { model_rate_limits: {
      'gpt-5': { rate_limited_at: '2026-09-21T00:00:00Z', rate_limit_reset_at: '2026-09-21T00:01:00Z' }
    } } }) }
    const first = mount(AccountStatusIndicator, { props, global: { stubs: { Icon: true } } })
    const second = mount(AccountStatusIndicator, { props, global: { stubs: { Icon: true } } })
    expect(vi.getTimerCount()).toBe(1)
    expect(first.text()).toContain('gpt-5')
    await vi.advanceTimersByTimeAsync(60_000)
    expect(first.text()).not.toContain('gpt-5')
    expect(second.text()).not.toContain('gpt-5')
    first.unmount()
    expect(vi.getTimerCount()).toBe(1)
    second.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
  it('Claude 5 模型限流时显示 Opus 和 Sonnet 的短别名', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          extra: {
            model_rate_limits: {
              'claude-opus-5': {
                rate_limited_at: '2026-07-28T00:00:00Z',
                rate_limit_reset_at: '2099-07-28T00:00:00Z'
              },
              'claude-sonnet-5': {
                rate_limited_at: '2026-07-28T00:00:00Z',
                rate_limit_reset_at: '2099-07-28T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('COpus5')
    expect(wrapper.text()).toContain('CSon5')
    expect(wrapper.text()).not.toContain('claude-sonnet-5')
  })

  it('Grok 账号额度限流时显示自动恢复时间而非临时不可调度', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 5,
          name: 'grok-free-1',
          platform: 'grok',
          rate_limited_at: '2026-07-11T12:00:00Z',
          rate_limit_reset_at: '2099-07-11T13:00:00Z',
          temp_unschedulable_until: '2099-07-11T12:30:00Z',
          temp_unschedulable_reason: 'legacy grok rate limited'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('.badge-warning').text()).toBe('admin.accounts.status.rateLimited')
    expect(wrapper.text()).toContain('admin.accounts.status.rateLimitedAutoResume')
    expect(wrapper.text()).not.toContain('admin.accounts.status.tempUnschedulable')
  })

  it('模型限流 + overages 启用 + 无 AICredits key → 显示 ⚡ (credits_active)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 1,
          name: 'ag-1',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('⚡')
    expect(wrapper.text()).toContain('CSon45')
  })

  it('模型限流 + overages 未启用 → 普通限流样式（无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 2,
          name: 'ag-2',
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
  })

  it('AICredits key 生效 → 显示积分已用尽 (credits_exhausted)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 3,
          name: 'ag-3',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('模型限流 + overages 启用 + AICredits key 生效 → 普通限流样式（积分耗尽，无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 4,
          name: 'ag-4',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              },
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    // 模型限流 + 积分耗尽 → 不应显示 ⚡
    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
    // AICredits 积分耗尽状态应显示
    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })
})
