import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AccountTableFilters from '../AccountTableFilters.vue'
import Select from '@/components/common/Select.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

enableAutoUnmount(afterEach)
afterEach(() => { document.body.innerHTML = '' })

describe('AccountTableFilters', () => {
  it('keeps CPR selectable in the inline filters without clearing other selections', async () => {
    const filters = {
      platform: 'openai', type: '', status: 'active', privacy_mode: '__unset__',
      group: '42', search: 'existing search', sort_by: 'priority', sort_order: 'asc'
    }
    const wrapper = mount(AccountTableFilters, {
      attachTo: document.body,
      props: { searchQuery: filters.search, filters },
      global: { stubs: { Teleport: true } }
    })
    const selects = wrapper.findAllComponents(Select)
    expect(selects).toHaveLength(5)
    expect(selects.every(select => select.isVisible())).toBe(true)
    const typeSelect = selects.find(select => select.props('options').some(option => option.value === 'cpr'))
    expect(typeSelect).toBeDefined()
    await typeSelect!.get('button').trigger('click')
    const cprOption = typeSelect!.findAll('[role="option"]').find(option => option.text() === 'CPR')
    expect(cprOption).toBeDefined()
    await cprOption!.trigger('click')
    expect(wrapper.emitted('update:filters')).toEqual([[{ ...filters, type: 'cpr' }]])
    expect(wrapper.emitted('change')).toHaveLength(1)
    expect(wrapper.props('filters')).toEqual(filters)
  })
})
