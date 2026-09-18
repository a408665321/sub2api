<template>
  <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-600" :aria-label="userId ? text.userTrend : text.siteTrend">
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <h3 class="mr-auto text-sm font-semibold">{{ userId ? text.userTrend : text.siteTrend }}</h3>
      <select v-model="period" class="input w-auto" :aria-label="text.period" data-testid="trend-period">
        <option value="day">{{ text.day }}</option><option value="month">{{ text.monthly }}</option>
      </select>
      <select v-model="metric" class="input w-auto" :aria-label="text.metric" data-testid="trend-metric">
        <option v-for="key in metrics" :key="key" :value="key">{{ text[key] }}</option>
      </select>
    </div>
    <div v-if="loading" class="flex h-52 items-center justify-center text-sm" role="status">{{ text.loading }}</div>
    <div v-else-if="failed" class="flex h-52 items-center justify-center gap-3" role="alert">
      {{ text.failed }} <button class="btn btn-secondary" data-testid="retry-trend" @click="load">{{ text.retry }}</button>
    </div>
    <template v-else-if="result">
      <p v-if="result.history_incomplete" class="mb-3 text-xs text-amber-700 dark:text-amber-400">{{ text.history }}</p>
      <div v-if="result.points.some(p => p.available)" class="h-52">
        <Line :data="chartData" :options="options" />
      </div>
      <p v-else class="flex h-52 items-center justify-center text-sm text-gray-500">{{ text.empty }}</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler } from 'chart.js'
import type { ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import { getUsageReportTrend } from '@/api/admin/photonthinxUsageReports'
import type { ReportMetric, UsageReportTrend } from '@/types/photonthinxUsageReports'
import { reportMoney, reportNumber, usePhotonthinxReportText } from './photonthinxReportText'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)
const props = defineProps<{ month: string; userId?: number }>()
const text = usePhotonthinxReportText()
const metrics: ReportMetric[] = ['actual_cost', 'requests', 'total_tokens']
const metric = ref<ReportMetric>('actual_cost')
const period = ref<'day' | 'month'>('day')
const result = ref<UsageReportTrend | null>(null)
const loading = ref(false)
const failed = ref(false)
let controller: AbortController | undefined
async function load() {
  controller?.abort()
  const request = new AbortController(); controller = request
  loading.value = true; failed.value = false; result.value = null
  try {
    const data = await getUsageReportTrend({ month: props.month, user_id: props.userId, granularity: period.value, months: period.value === 'day' ? 1 : 12 }, request.signal)
    if (controller === request && !request.signal.aborted) result.value = data
  } catch {
    if (controller === request && !request.signal.aborted) failed.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}
watch(() => [props.month, props.userId, period.value], load, { immediate: true })
onUnmounted(() => controller?.abort())
const chartData = computed(() => ({
  labels: result.value?.points.map(p => p.date) ?? [],
  datasets: [{ label: text.value[metric.value], data: result.value?.points.map(p => p.available ? p[metric.value] : null) ?? [], borderColor: '#6366f1', backgroundColor: '#6366f118', fill: true, spanGaps: false, tension: 0.15, pointRadius: 3 }]
}))
const options = computed<ChartOptions<'line'>>(() => ({
  responsive: true, maintainAspectRatio: false, interaction: { intersect: false, mode: 'index' },
  plugins: { legend: { display: false }, tooltip: { callbacks: {
    label: ctx => `${text.value[metric.value]}: ${metric.value === 'actual_cost' ? reportMoney(ctx.parsed.y ?? 0) : reportNumber(ctx.parsed.y ?? 0)}`,
    afterLabel: ctx => result.value?.points[ctx.dataIndex]?.partial ? text.value.partial : ''
  } } },
  scales: { y: { beginAtZero: true, ticks: { precision: metric.value === 'actual_cost' ? undefined : 0 } } }
}))
</script>
