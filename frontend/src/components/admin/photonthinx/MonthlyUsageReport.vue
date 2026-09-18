<template>
  <section class="space-y-5 p-4 text-gray-900 dark:text-gray-100 sm:p-6" :aria-label="text.tab">
    <div class="flex flex-wrap items-end gap-4">
      <div>
        <label for="photonthinx-month" class="mb-1 block text-sm font-medium">{{ text.month }}</label>
        <div class="flex items-center gap-2">
          <button class="btn btn-secondary" data-testid="previous-month" :disabled="month <= '2000-01'" :aria-label="text.previous" @click="month = shiftMonth(month, -1)">‹</button>
          <input id="photonthinx-month" v-model="month" type="month" min="2000-01" :max="currentMonth" class="input w-auto" />
          <button class="btn btn-secondary" :disabled="month >= currentMonth" :aria-label="text.next" @click="month = shiftMonth(month, 1)">›</button>
        </div>
      </div>
      <form class="flex flex-1 items-center gap-2 sm:max-w-md" @submit.prevent="applySearch">
        <input v-model="searchInput" type="search" maxlength="200" class="input min-w-0 flex-1" :placeholder="text.search" :aria-label="text.search" />
        <button type="submit" class="btn btn-secondary">{{ text.searchButton }}</button>
      </form>
    </div>
    <div class="space-y-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
      <p>{{ text.basis }}</p><p>{{ text.rules }}</p>
    </div>
    <p v-if="loading" class="py-16 text-center text-sm" role="status">{{ text.loading }}</p>
    <div v-else-if="failed" class="flex items-center justify-center gap-3 py-16" role="alert">
      {{ text.failed }} <button class="btn btn-secondary" data-testid="retry-report" @click="load">{{ text.retry }}</button>
    </div>
    <template v-else-if="report">
      <div class="flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span v-if="report.current" class="rounded-full bg-primary-50 px-2 py-1 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ text.current }} · {{ reportTimestamp(report.as_of) }}</span>
        <span v-if="report.available_from" class="py-1">{{ text.available }}: {{ reportTimestamp(report.available_from) }}</span>
      </div>
      <p v-if="report.history_incomplete" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ text.history }}</p>
      <div data-testid="site-summary" class="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <div v-for="key in summaryKeys" :key="key" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ text[key] }}</div>
          <div class="mt-2 break-words text-xl font-semibold tabular-nums">{{ key.endsWith('cost') ? reportMoney(report.summary[key]) : reportNumber(report.summary[key]) }}</div>
        </div>
      </div>
      <PhotonthinxUsageTrend :key="month" :month="month" />
      <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
        <table class="w-full whitespace-nowrap text-left text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th scope="col" class="p-3">#</th><th scope="col" class="p-3">{{ text.user }}</th>
              <th scope="col" class="p-3 text-right" :aria-sort="ariaSort('actual_cost')"><button data-testid="sort-actual_cost" @click="sort('actual_cost')">{{ text.actual_cost }} {{ sortArrow('actual_cost') }}</button></th>
              <th scope="col" class="p-3 text-right">{{ text.share }}</th>
              <th scope="col" class="p-3 text-right" :aria-sort="ariaSort('requests')"><button data-testid="sort-requests" @click="sort('requests')">{{ text.requests }} {{ sortArrow('requests') }}</button></th>
              <th scope="col" class="p-3 text-right">{{ text.input }}</th><th scope="col" class="p-3 text-right">{{ text.output }}</th>
              <th scope="col" class="p-3 text-right">{{ text.cacheCreation }}</th><th scope="col" class="p-3 text-right">{{ text.cacheRead }}</th>
              <th scope="col" class="p-3 text-right" :aria-sort="ariaSort('total_tokens')"><button data-testid="sort-total_tokens" @click="sort('total_tokens')">{{ text.total_tokens }} {{ sortArrow('total_tokens') }}</button></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <template v-for="(user, index) in report.items" :key="user.user_id">
              <tr class="hover:bg-gray-50 dark:hover:bg-dark-800/50">
                <td class="p-3 text-gray-400">{{ (page - 1) * pageSize + index + 1 }}</td>
                <td class="p-3">
                  <button :data-testid="`expand-${user.user_id}`" :aria-expanded="expanded === user.user_id" :aria-label="`${expanded === user.user_id ? text.collapse : text.details}: ${user.email || user.user_id}`" class="text-left text-primary-600 dark:text-primary-400" @click="expanded = expanded === user.user_id ? null : user.user_id">
                    <span class="block font-medium">{{ expanded === user.user_id ? '▾' : '▸' }} {{ user.username || user.email || `ID ${user.user_id}` }}</span>
                    <span class="block text-xs text-gray-500">{{ user.username ? user.email : '' }} · ID {{ user.user_id }}</span>
                  </button>
                </td>
                <td class="p-3 text-right font-medium tabular-nums">{{ reportMoney(user.actual_cost) }}</td>
                <td class="p-3 text-right tabular-nums">{{ report.summary.actual_cost === 0 ? '—' : `${(user.cost_share * 100).toFixed(2)}%` }}</td>
                <td v-for="key in tokenKeys" :key="key" class="p-3 text-right tabular-nums">{{ reportNumber(user[key]) }}</td>
              </tr>
              <tr v-if="expanded === user.user_id">
                <td colspan="10" class="bg-gray-50 p-4 dark:bg-dark-800/40">
                  <PhotonthinxUsageTrend :key="`${month}-${user.user_id}`" :month="month" :user-id="user.user_id" />
                </td>
              </tr>
            </template>
            <tr v-if="report.items.length === 0"><td colspan="10" class="p-10 text-center text-gray-500">{{ text.empty }}</td></tr>
          </tbody>
        </table>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3 text-sm">
        <span>{{ report.total }} {{ text.users }} · {{ page }} / {{ pages }} {{ text.page }}</span>
        <div class="flex items-center gap-2">
          <select v-model.number="pageSize" class="input w-auto" :aria-label="text.pageSize"><option :value="20">20</option><option :value="50">50</option><option :value="100">100</option></select>
          <button class="btn btn-secondary" :disabled="page <= 1" @click="page--">{{ text.previousPage }}</button>
          <button class="btn btn-secondary" data-testid="next-page" :disabled="page >= pages" @click="page++">{{ text.nextPage }}</button>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { getMonthlyUsageReport } from '@/api/admin/photonthinxUsageReports'
import type { MonthlyUsageReport, ReportMetric } from '@/types/photonthinxUsageReports'
import PhotonthinxUsageTrend from './PhotonthinxUsageTrend.vue'
import { reportMoney, reportNumber, reportTimestamp, shanghaiMonth, shiftMonth, usePhotonthinxReportText } from './photonthinxReportText'

const text = usePhotonthinxReportText()
const currentMonth = shanghaiMonth()
const month = ref(currentMonth)
const searchInput = ref('')
const search = ref('')
const page = ref(1)
const pageSize = ref(20)
const sortBy = ref<ReportMetric>('actual_cost')
const sortOrder = ref<'asc' | 'desc'>('desc')
const expanded = ref<number | null>(null)
const report = ref<MonthlyUsageReport | null>(null)
const loading = ref(false)
const failed = ref(false)
const pages = computed(() => Math.max(1, Math.ceil((report.value?.total ?? 0) / pageSize.value)))
const summaryKeys = ['actual_cost', 'account_cost', 'requests', 'total_tokens', 'active_users'] as const
const tokenKeys = ['requests', 'input_tokens', 'output_tokens', 'cache_creation_tokens', 'cache_read_tokens', 'total_tokens'] as const
let controller: AbortController | undefined
function applySearch() { search.value = searchInput.value.trim(); page.value = 1 }
function sort(key: ReportMetric) {
  sortOrder.value = sortBy.value === key && sortOrder.value === 'desc' ? 'asc' : 'desc'
  sortBy.value = key; page.value = 1
}
const ariaSort = (key: ReportMetric) => sortBy.value !== key ? 'none' : sortOrder.value === 'desc' ? 'descending' : 'ascending'
const sortArrow = (key: ReportMetric) => sortBy.value === key ? (sortOrder.value === 'desc' ? '↓' : '↑') : '↕'
async function load() {
  controller?.abort()
  const request = new AbortController(); controller = request
  expanded.value = null; loading.value = true; failed.value = false; report.value = null
  try {
    const data = await getMonthlyUsageReport({ month: month.value, search: search.value, page: page.value, page_size: pageSize.value, sort_by: sortBy.value, sort_order: sortOrder.value }, request.signal)
    if (controller === request && !request.signal.aborted) report.value = data
  } catch {
    if (controller === request && !request.signal.aborted) failed.value = true
  } finally { if (controller === request) loading.value = false }
}
watch([month, search, pageSize], () => { page.value = 1 })
watch([month, search, page, pageSize, sortBy, sortOrder], load, { immediate: true })
onUnmounted(() => controller?.abort())
</script>
