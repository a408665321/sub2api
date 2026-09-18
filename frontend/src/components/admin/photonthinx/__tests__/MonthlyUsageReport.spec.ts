import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MonthlyUsageReport from '../MonthlyUsageReport.vue'
import PhotonthinxUsageTrend from '../PhotonthinxUsageTrend.vue'

const { monthly, trend } = vi.hoisted(() => ({ monthly: vi.fn(), trend: vi.fn() }))
vi.mock('@/api/admin/photonthinxUsageReports', () => ({ getMonthlyUsageReport: monthly, getUsageReportTrend: trend }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh' } }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<div />' } }))
const metrics = { actual_cost: 2, account_cost: 1, requests: 3, input_tokens: 4, output_tokens: 5, cache_creation_tokens: 6, cache_read_tokens: 7, total_tokens: 22 }
const data = (email = 'first@example.com') => ({
  month: '2026-09', summary: { ...metrics, active_users: 2 },
  items: [1, 2].map(user_id => ({ ...metrics, user_id, email: user_id === 1 ? email : 'second@example.com', username: '', cost_share: 0.5 })),
  total: 41, page: 1, page_size: 20, timezone: 'Asia/Shanghai', as_of: '2026-09-18T10:00:00+08:00',
  available_from: '2026-09-03T13:15:00+08:00', history_incomplete: true, current: true
})
const render = () => mount(MonthlyUsageReport, { global: { stubs: { PhotonthinxUsageTrend: true } } })
describe('Photonthinx monthly report', () => {
  beforeEach(() => { monthly.mockReset().mockResolvedValue(data()); trend.mockReset().mockResolvedValue({ points: [], history_incomplete: true }) })
  it('loads cost ranking, keeps global summary, and only expands one user', async () => {
    const w = render(); await flushPromises()
    expect(monthly).toHaveBeenCalledWith(expect.objectContaining({ sort_by: 'actual_cost', sort_order: 'desc', page: 1 }), expect.any(AbortSignal))
    expect(w.text()).toContain('数据不足'); expect(w.text()).toContain('截至当前')
    expect(w.findAllComponents(PhotonthinxUsageTrend)).toHaveLength(1)
    await w.get('[data-testid="expand-1"]').trigger('click')
    expect(w.findAllComponents(PhotonthinxUsageTrend)).toHaveLength(2)
    expect(w.findAllComponents(PhotonthinxUsageTrend)[1].props('userId')).toBe(1)
    await w.get('[data-testid="expand-2"]').trigger('click')
    expect(w.findAllComponents(PhotonthinxUsageTrend)).toHaveLength(2)
    expect(w.findAllComponents(PhotonthinxUsageTrend)[1].props('userId')).toBe(2)
    await w.get('[data-testid="next-page"]').trigger('click'); await flushPromises()
    expect(monthly).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }), expect.any(AbortSignal))
    await w.get('[data-testid="sort-requests"]').trigger('click'); await flushPromises()
    expect(monthly).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, sort_by: 'requests', sort_order: 'desc' }), expect.any(AbortSignal))
    await w.get('input[type="search"]').setValue('second'); await w.get('form').trigger('submit'); await flushPromises()
    expect(monthly).toHaveBeenLastCalledWith(expect.objectContaining({ search: 'second' }), expect.any(AbortSignal))
    expect(w.get('[data-testid="site-summary"]').text()).toContain('账号成本')
    w.unmount()
  })
  it('month switching ignores an older response and resets expansion', async () => {
    let resolve!: (value: ReturnType<typeof data>) => void
    monthly.mockImplementationOnce(() => new Promise(r => { resolve = r }))
    const w = render()
    expect(w.text()).toContain('加载中')
    await w.get('input[type="month"]').setValue('2024-02'); await flushPromises()
    expect(monthly).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2024-02' }), expect.any(AbortSignal))
    resolve(data('stale@example.com')); await flushPromises()
    expect(w.text()).not.toContain('stale@example.com')
    await w.get('[data-testid="previous-month"]').trigger('click'); await flushPromises()
    expect(monthly).toHaveBeenLastCalledWith(expect.objectContaining({ month: '2024-01' }), expect.any(AbortSignal))
    w.unmount()
  })
  it('failure shows retry instead of a zero report; empty retained data is explicit', async () => {
    monthly.mockRejectedValueOnce(new Error('unavailable'))
    const w = render(); await flushPromises()
    expect(w.find('[data-testid="site-summary"]').exists()).toBe(false)
    await w.get('[data-testid="retry-report"]').trigger('click'); await flushPromises()
    expect(w.find('[data-testid="site-summary"]').exists()).toBe(true)
    monthly.mockResolvedValue({ ...data(), items: [], total: 0 })
    await w.get('[data-testid="previous-month"]').trigger('click'); await flushPromises()
    expect(w.text()).toContain('没有匹配的用量记录'); w.unmount()
  })
})

describe('Photonthinx report trends', () => {
  beforeEach(() => trend.mockReset().mockResolvedValue({ points: [{ date: '2026-09-01', available: false, ...metrics }, { date: '2026-09-03', available: true, ...metrics }], history_incomplete: true }))
  const renderTrend = () => mount(PhotonthinxUsageTrend, { props: { month: '2026-09', userId: 1 } })
  it('loads day/month and metric views; unavailable history produces gaps', async () => {
    const w = renderTrend(); await flushPromises()
    expect(trend).toHaveBeenCalledWith({ month: '2026-09', user_id: 1, granularity: 'day', months: 1 }, expect.any(AbortSignal))
    expect(w.findComponent({ name: 'Line' }).props('data').datasets[0].data).toEqual([null, 2])
    await w.get('[data-testid="trend-metric"]').setValue('requests')
    expect(w.findComponent({ name: 'Line' }).props('data').datasets[0].data).toEqual([null, 3])
    await w.get('[data-testid="trend-period"]').setValue('month'); await flushPromises()
    expect(trend).toHaveBeenLastCalledWith(expect.objectContaining({ granularity: 'month', months: 12 }), expect.any(AbortSignal))
    w.unmount()
  })
  it('ignores stale user trends and retries failed queries', async () => {
    let resolve!: (v: unknown) => void
    trend.mockImplementationOnce(() => new Promise(r => { resolve = r }))
    const w = renderTrend(); await w.setProps({ userId: 2 }); await flushPromises()
    resolve({ points: [{ date: 'stale', available: true, ...metrics }] }); await flushPromises()
    expect(w.findComponent({ name: 'Line' }).props('data').labels).not.toContain('stale')
    trend.mockRejectedValueOnce(new Error('failed')); await w.setProps({ month: '2026-08' }); await flushPromises()
    expect(w.findComponent({ name: 'Line' }).exists()).toBe(false)
    await w.get('[data-testid="retry-trend"]').trigger('click'); await flushPromises()
    expect(w.findComponent({ name: 'Line' }).exists()).toBe(true); w.unmount()
  })
})
