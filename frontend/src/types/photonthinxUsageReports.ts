export type ReportMetric = 'actual_cost' | 'requests' | 'total_tokens'
export interface ReportMetrics {
  actual_cost: number
  account_cost: number
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
}
export interface ReportMetadata {
  available_from: string | null
  history_incomplete: boolean
  current: boolean
  as_of: string
  timezone: string
}
export interface ReportUser extends ReportMetrics {
  user_id: number
  username: string
  email: string
  cost_share: number
}
export interface MonthlyUsageReport extends ReportMetadata {
  month: string
  summary: ReportMetrics & { active_users: number }
  items: ReportUser[]
  total: number
  page: number
  page_size: number
}
export interface ReportPoint extends ReportMetrics {
  date: string
  available: boolean
  partial: boolean
}
export interface UsageReportTrend extends ReportMetadata { points: ReportPoint[] }
export interface MonthlyReportQuery {
  month: string
  search: string
  page: number
  page_size: number
  sort_by: ReportMetric
  sort_order: 'asc' | 'desc'
}
export interface ReportTrendQuery {
  month: string
  user_id?: number
  granularity: 'day' | 'month'
  months: number
}
