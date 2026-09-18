import { apiClient } from '../client'
import type { MonthlyReportQuery, MonthlyUsageReport, ReportTrendQuery, UsageReportTrend } from '@/types/photonthinxUsageReports'

const base = '/admin/internal/reports/usage'
export async function getMonthlyUsageReport(params: MonthlyReportQuery, signal: AbortSignal): Promise<MonthlyUsageReport> {
  const { data } = await apiClient.get<MonthlyUsageReport>(`${base}/monthly`, { params, signal, timeout: 35000 })
  return data
}
export async function getUsageReportTrend(params: ReportTrendQuery, signal: AbortSignal): Promise<UsageReportTrend> {
  const { data } = await apiClient.get<UsageReportTrend>(`${base}/trend`, { params, signal, timeout: 35000 })
  return data
}
