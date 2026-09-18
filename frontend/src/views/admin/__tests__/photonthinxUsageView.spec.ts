import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import UsageView from '../UsageView.vue'

vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  usage: { list: vi.fn().mockResolvedValue({ items: [], total: 0 }), getStats: vi.fn().mockResolvedValue({}) },
  dashboard: { getSnapshotV2: vi.fn().mockResolvedValue({}), getModelStats: vi.fn().mockResolvedValue({ models: [] }) }
} }))
vi.mock('@/api/admin/photonthinxUsageReports', () => ({ getMonthlyUsageReport: vi.fn(), getUsageReportTrend: vi.fn() }))
const Filters = defineComponent({ setup() { return { value: ref('original user filter') } }, template: '<div><input v-model="value" /></div>' })
describe('Photonthinx usage integration', () => {
  it('lazily opens monthly reports and preserves original filter UI state', async () => {
    const w = mount(UsageView, { global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' }, UsageStatsCards: true, UsageFilters: Filters,
      UsageTable: true, UsageExportProgress: true, UsageCleanupDialog: true, UserBalanceHistoryModal: true,
      Pagination: true, Select: true, DateRangePicker: true, Icon: true, TokenUsageTrend: true,
      ModelDistributionChart: true, GroupDistributionChart: true, EndpointDistributionChart: true,
      UserTokenRanking: true, OpsErrorLogTable: true, OpsErrorDetailModal: true, PhotonthinxMonthlyReport: true,
    } } })
    await flushPromises()
    expect(w.findComponent({ name: 'PhotonthinxMonthlyReport' }).exists()).toBe(false)
    const filters = w.findComponent(Filters)
    await filters.get('input').setValue('my retained filter')
    const tabs = w.findAll('[data-testid="usage-detail-tab"]')
    expect(tabs).toHaveLength(3)
    await w.get('[data-testid="photonthinx-monthly-tab"]').trigger('click'); await flushPromises()
    expect((filters.element as HTMLElement).style.display).toBe('none')
    expect(w.find('usage-stats-cards-stub').isVisible()).toBe(false)
    expect(w.find('photonthinx-monthly-report-stub').exists()).toBe(true)
    await tabs[0].trigger('click'); await flushPromises()
    expect((filters.element as HTMLElement).style.display).not.toBe('none')
    expect((filters.get('input').element as HTMLInputElement).value).toBe('my retained filter')
    w.unmount()
  })
})
