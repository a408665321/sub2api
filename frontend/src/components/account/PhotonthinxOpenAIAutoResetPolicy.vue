<template>
  <div class="space-y-4 border-t border-gray-200 pt-4 dark:border-dark-600" data-testid="auto-reset-credit-settings">
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <label class="input-label mb-0">{{ t('admin.accounts.autoResetCredit.title') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.autoResetCredit.hint') }}</p>
      </div>
      <button
        type="button"
        role="switch"
        :aria-checked="enabled"
        data-testid="auto-reset-credit-enabled"
        :class="toggleClass(enabled)"
        @click="emit('update:enabled', !enabled)"
      >
        <span :class="toggleKnobClass(enabled)" />
      </button>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.accounts.autoResetCredit.mode') }}</label>
        <select
          :value="mode"
          class="input"
          :disabled="!enabled"
          data-testid="auto-reset-policy-mode"
          @change="handleModeChange"
        >
          <option value="observe">{{ t('admin.accounts.autoResetCredit.observe') }}</option>
          <option value="enforce">{{ t('admin.accounts.autoResetCredit.enforce') }}</option>
        </select>
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.autoResetCredit.guardDays') }}</label>
        <input
          :value="guardDays"
          type="number"
          min="0"
          max="7"
          step="0.5"
          class="input"
          :disabled="!enabled || !reset7dEnabled"
          data-testid="auto-reset-policy-guard-days"
          @input="emitNumber('update:guardDays', $event)"
        />
      </div>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-3">
          <label class="input-label mb-0">{{ t('admin.accounts.autoResetCredit.reset5hEnabled') }}</label>
          <button
            type="button"
            role="switch"
            :aria-checked="reset5hEnabled"
            :disabled="!enabled"
            data-testid="auto-reset-policy-5h-enabled"
            :class="toggleClass(reset5hEnabled, !enabled)"
            @click="emit('update:reset5hEnabled', !reset5hEnabled)"
          >
            <span :class="toggleKnobClass(reset5hEnabled)" />
          </button>
        </div>
        <label class="input-label">{{ t('admin.accounts.autoResetCredit.threshold5h') }}</label>
        <input
          :value="threshold5h"
          type="number"
          min="0.1"
          max="100"
          step="0.1"
          class="input"
          :disabled="!enabled || !reset5hEnabled"
          data-testid="auto-reset-credit-5h-threshold"
          @input="emitNumber('update:threshold5h', $event)"
        />
      </div>
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-3">
          <label class="input-label mb-0">{{ t('admin.accounts.autoResetCredit.reset7dEnabled') }}</label>
          <button
            type="button"
            role="switch"
            :aria-checked="reset7dEnabled"
            :disabled="!enabled"
            data-testid="auto-reset-policy-7d-enabled"
            :class="toggleClass(reset7dEnabled, !enabled)"
            @click="emit('update:reset7dEnabled', !reset7dEnabled)"
          >
            <span :class="toggleKnobClass(reset7dEnabled)" />
          </button>
        </div>
        <label class="input-label">{{ t('admin.accounts.autoResetCredit.threshold7d') }}</label>
        <input
          :value="threshold7d"
          type="number"
          min="0.1"
          max="100"
          step="0.1"
          class="input"
          :disabled="!enabled || !reset7dEnabled"
          data-testid="auto-reset-credit-7d-threshold"
          @input="emitNumber('update:threshold7d', $event)"
        />
      </div>
    </div>

    <p class="rounded-md bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-700 dark:bg-dark-700 dark:text-gray-300" data-testid="auto-reset-policy-summary">
      {{ strategySummary }}
    </p>

    <div v-if="presence || latestDecisions.length" class="grid gap-3 text-xs sm:grid-cols-2">
      <div v-for="window in ['5h', '7d']" :key="window" class="space-y-1">
        <div class="font-medium text-gray-700 dark:text-gray-300">
          {{ window }}: {{ presenceLabel(window) }}
        </div>
        <div v-if="decisionFor(window)" class="text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.autoResetCredit.latestDecision', {
            decision: decisionLabel(decisionFor(window)?.decision),
            reason: reasonLabel(decisionFor(window)?.reason),
            time: decisionFor(window)?.evaluated_at
          }) }}
        </div>
      </div>
    </div>
  </div>

  <ConfirmDialog
    :show="showEnforceConfirm"
    :title="t('admin.accounts.autoResetCredit.enforceConfirmTitle')"
    :message="t('admin.accounts.autoResetCredit.enforceConfirmMessage', { summary: strategySummary })"
    :confirm-text="t('admin.accounts.autoResetCredit.enforce')"
    :danger="true"
    @confirm="confirmEnforce"
    @cancel="showEnforceConfirm = false"
  />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import type { PhotonthinxAutoResetObservationState, PhotonthinxCodexWindowPresence } from '@/types'

const props = defineProps<{
  enabled: boolean
  threshold5h: number
  threshold7d: number
  mode: 'observe' | 'enforce'
  reset5hEnabled: boolean
  reset7dEnabled: boolean
  guardDays: number
  autoPause5hThreshold: number | null
  autoPause7dThreshold: number | null
  presence?: PhotonthinxCodexWindowPresence
  observationState?: PhotonthinxAutoResetObservationState
}>()

const emit = defineEmits<{
  (event: 'update:enabled', value: boolean): void
  (event: 'update:threshold5h', value: number): void
  (event: 'update:threshold7d', value: number): void
  (event: 'update:mode', value: 'observe' | 'enforce'): void
  (event: 'update:reset5hEnabled', value: boolean): void
  (event: 'update:reset7dEnabled', value: boolean): void
  (event: 'update:guardDays', value: number): void
}>()

const { t } = useI18n()
const showEnforceConfirm = ref(false)

const toggleClass = (active: boolean, disabled = false) => [
  'relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
  disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
  active ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
]
const toggleKnobClass = (active: boolean) => [
  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200',
  active ? 'translate-x-5' : 'translate-x-0'
]

const strategySummary = computed(() => {
  const pause5h = props.autoPause5hThreshold == null
    ? t('admin.accounts.autoResetCredit.globalPauseThreshold')
    : `${props.autoPause5hThreshold}%`
  const pause7d = props.autoPause7dThreshold == null
    ? t('admin.accounts.autoResetCredit.globalPauseThreshold')
    : `${props.autoPause7dThreshold}%`
  return t('admin.accounts.autoResetCredit.summary', {
    mode: t(`admin.accounts.autoResetCredit.${props.mode}`),
    fiveHour: props.reset5hEnabled
      ? t('admin.accounts.autoResetCredit.summaryReset', { threshold: props.threshold5h })
      : t('admin.accounts.autoResetCredit.summaryNeverReset', { threshold: pause5h }),
    sevenDay: props.reset7dEnabled
      ? t('admin.accounts.autoResetCredit.summaryGuard', { threshold: props.threshold7d, days: props.guardDays, pause: pause7d })
      : t('admin.accounts.autoResetCredit.summaryNeverReset', { threshold: pause7d })
  })
})

const latestDecisions = computed(() => Object.values(props.observationState?.windows ?? {}))
const decisionFor = (window: string) => props.observationState?.windows?.[window]
const decisionLabel = (decision?: string) => {
  if (!decision) return t('admin.accounts.autoResetCredit.notAvailable')
  if (['would_bypass_pause', 'would_reset', 'skipped'].includes(decision)) {
    return t(`admin.accounts.autoResetCredit.decisions.${decision}`)
  }
  return decision
}
const reasonLabel = (reason?: string) => {
  if (!reason) return t('admin.accounts.autoResetCredit.notAvailable')
  if ([
    'threshold_reached', 'eligible_credit', 'window_disabled', 'window_absent',
    'natural_reset_guard', 'no_credit', 'invalid_window_signal',
    'credit_details_incomplete', 'credit_details_invalid'
  ].includes(reason)) {
    return t(`admin.accounts.autoResetCredit.reasons.${reason}`)
  }
  return reason
}
const presenceLabel = (window: string) => {
  const value = props.presence?.[window]
  if (!value || typeof value.present !== 'boolean') return t('admin.accounts.autoResetCredit.windowUnknown')
  return value.present
    ? t('admin.accounts.autoResetCredit.windowPresent')
    : t('admin.accounts.autoResetCredit.windowAbsent')
}

const emitNumber = (event: 'update:threshold5h' | 'update:threshold7d' | 'update:guardDays', rawEvent: Event) => {
  const value = Number((rawEvent.target as HTMLInputElement).value)
  if (event === 'update:threshold5h') emit('update:threshold5h', value)
  if (event === 'update:threshold7d') emit('update:threshold7d', value)
  if (event === 'update:guardDays') emit('update:guardDays', value)
}

const handleModeChange = (event: Event) => {
  const value = (event.target as HTMLSelectElement).value as 'observe' | 'enforce'
  if (value === 'enforce' && props.mode !== 'enforce') {
    showEnforceConfirm.value = true
    return
  }
  emit('update:mode', value)
}

const confirmEnforce = () => {
  showEnforceConfirm.value = false
  emit('update:mode', 'enforce')
}
</script>
