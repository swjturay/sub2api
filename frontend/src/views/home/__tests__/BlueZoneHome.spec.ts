import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { nextTick } from 'vue'

import BlueZoneHome from '../BlueZoneHome.vue'
import { BLUE_ZONE_CODE_DEMOS } from '../blueZoneHomeContent'

const messages: Record<string, string> = {
  'home.blueZone.brand': '蓝区AI网关',
  'home.blueZone.login': '登录平台',
  'home.blueZone.register': '注册接入',
  'home.blueZone.hero.lineOne': '汇聚多元 AI',
  'home.blueZone.hero.lineTwo': '助力蓝区研发作业',
  'home.blueZone.scale.title': '规模化应用，持续运营',
  'home.blueZone.scale.requestsValue': '超 7 万',
  'home.blueZone.scale.requestsLabel': '日均请求量',
  'home.blueZone.scale.tokensValue': '超 100 亿',
  'home.blueZone.scale.tokensLabel': '日均 Token 用量',
  'home.blueZone.demo.prompts.debounce': '实现一个按键消抖与短按检测逻辑',
  'home.blueZone.demo.prompts.heartbeat': '实现一个无阻塞的状态灯心跳任务',
  'home.blueZone.demo.prompts.queue': '实现一个线程安全的生产者消费者队列',
  'home.blueZone.demo.prompts.backoff': '实现一个支持指数退避的请求函数',
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => messages[key] ?? key }),
  }
})

function mountHome(overrides: Partial<InstanceType<typeof BlueZoneHome>['$props']> = {}) {
  return mount(BlueZoneHome, {
    props: {
      isAuthenticated: false,
      dashboardPath: '/dashboard',
      isDark: false,
      registrationEnabled: true,
      showModelPlazaEntry: false,
      ...overrides,
    },
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
      },
    },
  })
}

describe('BlueZoneHome', () => {
  beforeEach(() => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList)
  })

  it('renders the approved fixed promotional content without department or user metrics', () => {
    const wrapper = mountHome()

    expect(wrapper.text()).toContain('汇聚多元 AI')
    expect(wrapper.text()).toContain('助力蓝区研发作业')
    expect(wrapper.text()).toContain('超 7 万')
    expect(wrapper.text()).toContain('超 100 亿')
    expect(wrapper.text()).not.toContain('已接入部门')
    expect(wrapper.text()).not.toContain('用户')
  })

  it('uses login and registration routes for anonymous visitors', () => {
    const wrapper = mountHome()
    const destinations = wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))

    expect(destinations).toContain('/login')
    expect(destinations).toContain('/register')
  })

  it('uses the supplied dashboard route and hides registration after sign-in', () => {
    const wrapper = mountHome({ isAuthenticated: true, dashboardPath: '/admin/dashboard' })
    const destinations = wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))

    expect(destinations).toContain('/admin/dashboard')
    expect(destinations).not.toContain('/register')
  })

  it('keeps the four code examples local and omits language names from prompts', () => {
    expect(BLUE_ZONE_CODE_DEMOS.map((demo) => demo.file)).toEqual([
      'button_debounce.c',
      'heartbeat.rs',
      'TaskQueue.java',
      'request_with_backoff.py',
    ])

    const prompts = BLUE_ZONE_CODE_DEMOS.map((demo) => messages[demo.promptKey])
    expect(prompts.every((prompt) => !/\b(?:C|Rust|Java|Python)\b/i.test(prompt))).toBe(true)
    expect(BLUE_ZONE_CODE_DEMOS[0].code).toContain('gpio_read')
    expect(BLUE_ZONE_CODE_DEMOS[1].code).toContain('embedded_hal')
    expect(BLUE_ZONE_CODE_DEMOS[2].code).toContain('BlockingQueue')
    expect(BLUE_ZONE_CODE_DEMOS[3].code).toContain('requests.get')
  })

  it('shows the complete first example when reduced motion is requested', async () => {
    const wrapper = mountHome()
    await nextTick()

    expect(wrapper.text()).toContain('button_debounce.c')
    expect(wrapper.text()).toContain('gpio_read')
    expect(wrapper.find('.blue-typing-caret').exists()).toBe(false)
  })
})
