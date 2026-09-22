import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import PhotonthinxOpenAIAutoResetPolicy from '../PhotonthinxOpenAIAutoResetPolicy.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: Boolean },
  template: '<div v-if="show" data-testid="confirm-dialog"><slot /><slot name="footer" /></div>'
})

const mountPolicy = () => mount(PhotonthinxOpenAIAutoResetPolicy, {
  props: {
    enabled: true,
    threshold5h: 100,
    threshold7d: 100,
    mode: 'observe',
    reset5hEnabled: false,
    reset7dEnabled: true,
    guardDays: 2,
    autoPause5hThreshold: 95,
    autoPause7dThreshold: 95,
    presence: {
      '5h': { present: false },
      '7d': { present: true, window_minutes: 10080 }
    }
  },
  global: { stubs: { BaseDialog: BaseDialogStub } }
})

describe('PhotonthinxOpenAIAutoResetPolicy', () => {
  it('requires confirmation before switching to enforce and disables the 5h threshold', async () => {
    const wrapper = mountPolicy()
    expect(wrapper.get('[data-testid="auto-reset-credit-5h-threshold"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="auto-reset-policy-summary"]').text()).toContain('admin.accounts.autoResetCredit.summary')

    await wrapper.get('[data-testid="auto-reset-policy-mode"]').setValue('enforce')
    expect(wrapper.emitted('update:mode')).toBeUndefined()
    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(true)

    const buttons = wrapper.findAll('[data-testid="confirm-dialog"] button')
    await buttons.at(-1)!.trigger('click')
    expect(wrapper.emitted('update:mode')).toEqual([['enforce']])
  })
})
