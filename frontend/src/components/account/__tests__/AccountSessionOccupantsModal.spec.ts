import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import AccountSessionOccupantsModal from '../AccountSessionOccupantsModal.vue'
import { adminAPI } from '@/api/admin'

vi.mock('@/api/admin', () => ({
  adminAPI: { accounts: { getSessionOccupants: vi.fn() } }
}))

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: {} }, missingWarn: false })

const BaseDialog = {
  props: ['show', 'title'],
  emits: ['close'],
  template: '<section v-if="show"><h1>{{ title }}</h1><slot /></section>'
}

describe('AccountSessionOccupantsModal', () => {
  it('loads and renders grouped occupants including unknown sessions', async () => {
    vi.mocked(adminAPI.accounts.getSessionOccupants).mockResolvedValue({
      account_id: 7,
      active_sessions: 3,
      occupants: [
        { user_id: 42, username: 'alice', email: 'alice@example.com', last_active: '2026-09-27T01:00:00Z', session_count: 2 },
        { user_id: null, last_active: '2026-09-27T00:59:00Z', session_count: 1 }
      ]
    })
    const wrapper = mount(AccountSessionOccupantsModal, {
      props: { show: true, account: { id: 7, name: 'A' } as never },
      global: {
        stubs: { BaseDialog },
        plugins: [i18n]
      }
    })
    await flushPromises()
    expect(adminAPI.accounts.getSessionOccupants).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('admin.accounts.sessionOccupants.unknown')
    expect(wrapper.text()).toContain('2')
  })

  it('ignores a stale response after switching accounts', async () => {
    let resolveFirst!: (value: { account_id: number; active_sessions: number; occupants: never[] }) => void
    vi.mocked(adminAPI.accounts.getSessionOccupants)
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce({ account_id: 8, active_sessions: 0, occupants: [] })
    const wrapper = mount(AccountSessionOccupantsModal, {
      props: { show: true, account: { id: 7, name: 'A' } as never },
      global: { stubs: { BaseDialog }, plugins: [i18n] }
    })
    await wrapper.setProps({ account: { id: 8, name: 'B' } as never })
    await flushPromises()
    resolveFirst({ account_id: 7, active_sessions: 0, occupants: [] })
    await flushPromises()
    expect(adminAPI.accounts.getSessionOccupants).toHaveBeenLastCalledWith(8)
    expect(wrapper.text()).toContain('admin.accounts.sessionOccupants.empty')
  })

  it('supports error and retry states', async () => {
    vi.mocked(adminAPI.accounts.getSessionOccupants).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ account_id: 7, active_sessions: 0, occupants: [] })
    const wrapper = mount(AccountSessionOccupantsModal, {
      props: { show: true, account: { id: 7, name: 'A' } as never },
      global: { stubs: { BaseDialog }, plugins: [i18n] }
    })
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.sessionOccupants.error')
    await wrapper.get('[data-testid="session-occupants-refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.sessionOccupants.empty')
  })
})
