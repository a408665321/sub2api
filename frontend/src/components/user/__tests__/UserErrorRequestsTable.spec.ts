import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UserErrorRequestsTable from '../UserErrorRequestsTable.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key, te: () => false }) }
})
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))

const row = {
  id: 1,
  created_at: '2026-09-20',
  model: 'gpt-test',
  inbound_endpoint: '/v1/responses',
  status_code: 503,
  category: 'service_unavailable',
  platform: 'openai',
  message: 'Service temporarily unavailable account=secret',
  diagnosis_code: 'no_allocatable_resource',
  diagnosis_title: '暂无可用服务资源',
  diagnosis_reason: '当前没有可分配的渠道容量。',
  diagnosis_suggestion: '请稍后重试。',
  key_name: 'key',
  key_deleted: false,
}

describe('UserErrorRequestsTable diagnosis', () => {
  it('shows safe diagnosis instead of the raw message', () => {
    const wrapper = mount(UserErrorRequestsTable, {
      props: { rows: [row], total: 1, loading: false, page: 1, pageSize: 20 },
      global: {
        stubs: {
          DataTable: { props: ['data'], template: '<div><slot name="cell-message" :row="data[0]" /></div>' },
          IpGeoBatchToolbar: true,
          Pagination: true,
          UserErrorDetailModal: true,
        },
      },
    })
    expect(wrapper.text()).toContain('暂无可用服务资源')
    expect(wrapper.text()).toContain('当前没有可分配的渠道容量。')
    expect(wrapper.text()).not.toContain('account=secret')
  })
})
