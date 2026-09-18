import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const zh = {
  tab: '月度报表', month: '统计月份', previous: '上个月', next: '下个月', search: '搜索用户名称、邮箱或 ID', searchButton: '搜索',
  actual_cost: '实际费用', account_cost: '账号成本', requests: '请求次数', total_tokens: '总 Token', active_users: '活跃用户',
  user: '用户', share: '费用占比', input: '输入', output: '输出', cacheCreation: '缓存创建', cacheRead: '缓存读取',
  siteTrend: '全站使用趋势', userTrend: '用户使用趋势', day: '所选月份每日', monthly: '最近 12 个月', metric: '曲线指标', period: '曲线周期',
  loading: '加载中…', failed: '查询失败，请重试。', retry: '重试', empty: '没有匹配的用量记录', details: '展开趋势', collapse: '收起趋势',
  history: '数据不足：部分历史早于现存记录，曲线留空；首个有记录的周期可能不完整。',
  basis: '基于现存用量记录 · Asia/Shanghai（北京时间）· 金额单位 USD',
  rules: '请求次数为已保存的 API 用量记录数，不代表网页访问量或成功请求数。总 Token 包含输入、输出、缓存创建和缓存读取。搜索仅筛选用户列表，汇总及费用占比以全站整月为准。',
  available: '现存最早记录', current: '截至当前', partial: '周期未完整', previousPage: '上一页', nextPage: '下一页', page: '页', users: '位用户', pageSize: '每页用户数',
} as const
const en: Record<keyof typeof zh, string> = {
  tab: 'Monthly report', month: 'Month', previous: 'Previous month', next: 'Next month', search: 'Search name, email or ID', searchButton: 'Search',
  actual_cost: 'Actual fees', account_cost: 'Account cost', requests: 'Requests', total_tokens: 'Total tokens', active_users: 'Active users',
  user: 'User', share: 'Fee share', input: 'Input', output: 'Output', cacheCreation: 'Cache creation', cacheRead: 'Cache read',
  siteTrend: 'Site usage trend', userTrend: 'User usage trend', day: 'Daily in selected month', monthly: 'Last 12 months', metric: 'Metric', period: 'Period',
  loading: 'Loading…', failed: 'Query failed. Please retry.', retry: 'Retry', empty: 'No matching usage records', details: 'Expand trend', collapse: 'Collapse trend',
  history: 'Insufficient history: periods before retained records are left blank; the first recorded period may be incomplete.',
  basis: 'Based on retained usage records · Asia/Shanghai · Currency: USD',
  rules: 'Requests count stored API usage records, not page views or successful requests. Tokens include input, output, cache creation and cache read. Search only filters the user list; totals and fee shares use the whole site month.',
  available: 'Earliest retained record', current: 'As of now', partial: 'Partial period', previousPage: 'Previous', nextPage: 'Next', page: 'page', users: 'users', pageSize: 'Users per page',
}

export function usePhotonthinxReportText() {
  const { locale } = useI18n()
  return computed(() => locale?.value?.startsWith('en') ? en : zh)
}
export function shanghaiMonth(date = new Date()): string {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit' }).formatToParts(date)
  return `${parts.find(p => p.type === 'year')!.value}-${parts.find(p => p.type === 'month')!.value}`
}
export function shiftMonth(month: string, delta: number): string {
  const [year, m] = month.split('-').map(Number)
  return new Date(Date.UTC(year, m - 1 + delta, 1)).toISOString().slice(0, 7)
}
export const reportNumber = (value: number) => new Intl.NumberFormat('en-US').format(value)
export const reportMoney = (value: number) => new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 6 }).format(value)
export const reportTimestamp = (value: string) => new Intl.DateTimeFormat('zh-CN', { timeZone: 'Asia/Shanghai', dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
