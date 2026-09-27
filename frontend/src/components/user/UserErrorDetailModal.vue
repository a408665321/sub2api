<template>
  <BaseDialog
    :show="show"
    :title="t('usage.errors.detail.title')"
    width="wide"
    @close="emit('update:show', false)"
  >
    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-10">
      <svg
        class="h-7 w-7 animate-spin text-primary-500"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
    </div>

    <!-- Error state -->
    <div v-else-if="loadError" class="py-8 text-center text-sm text-red-500">
      {{ t("usage.errors.detail.loadFailed") }}
    </div>

    <!-- Detail content -->
    <div v-else-if="detail" class="space-y-4 text-sm">
      <div class="grid grid-cols-2 gap-x-6 gap-y-3">
        <!-- Time -->
        <div>
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.time")
          }}</span>
          <p class="mt-0.5 text-gray-900 dark:text-dark-100">
            {{ formatDateTime(detail.created_at) }}
          </p>
        </div>
        <!-- Model -->
        <div>
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.model")
          }}</span>
          <p class="mt-0.5 text-gray-900 dark:text-dark-100">
            {{ detail.model || "-" }}
          </p>
        </div>
        <!-- Endpoint -->
        <div>
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.endpoint")
          }}</span>
          <p class="mt-0.5 text-gray-900 dark:text-dark-100">
            {{ detail.inbound_endpoint || "-" }}
          </p>
        </div>
        <!-- Status Code -->
        <div>
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.status")
          }}</span>
          <p class="mt-0.5">
            <span class="badge" :class="statusClass(detail.status_code)">{{
              detail.status_code || "-"
            }}</span>
          </p>
        </div>
        <!-- Category -->
        <div>
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.category")
          }}</span>
          <p class="mt-0.5 text-gray-900 dark:text-dark-100">
            {{ t("usage.errors.categories." + detail.category) }}
          </p>
        </div>
        <!-- Platform -->
        <div>
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.platform")
          }}</span>
          <p class="mt-0.5 text-gray-900 dark:text-dark-100">
            {{ detail.platform || "-" }}
          </p>
        </div>
        <!-- Upstream status code -->
        <div v-if="detail.upstream_status_code != null">
          <span class="font-medium text-gray-500 dark:text-dark-400">{{
            t("usage.errors.detail.upstreamStatus")
          }}</span>
          <p class="mt-0.5 text-gray-900 dark:text-dark-100">
            {{ detail.upstream_status_code }}
          </p>
        </div>
      </div>

      <!-- Diagnosis -->
      <div
        class="rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-800 dark:bg-primary-950/30"
      >
        <h3 class="font-semibold text-gray-900 dark:text-white">
          {{ diagnosisText("title") }}
        </h3>
        <dl class="mt-3 space-y-3">
          <div>
            <dt class="text-xs font-semibold text-gray-500 dark:text-dark-400">
              {{ t("usage.errors.detail.reason") }}
            </dt>
            <dd class="mt-1 text-gray-800 dark:text-dark-200">
              {{ diagnosisText("reason") }}
            </dd>
          </div>
          <div>
            <dt class="text-xs font-semibold text-gray-500 dark:text-dark-400">
              {{ t("usage.errors.detail.suggestion") }}
            </dt>
            <dd class="mt-1 text-gray-800 dark:text-dark-200">
              {{ diagnosisText("suggestion") }}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import BaseDialog from "@/components/common/BaseDialog.vue";
import { getMyErrorDetail } from "@/api/usage";
import { formatDateTime } from "@/utils/format";
import type { UserErrorRequestDetail } from "@/types";

const props = defineProps<{
  show: boolean;
  errorId: number | null;
}>();

const emit = defineEmits<{
  (e: "update:show", v: boolean): void;
}>();

const { t, te } = useI18n();

function diagnosisText(field: "title" | "reason" | "suggestion"): string {
  if (!detail.value) return "";
  const key = `usage.errors.diagnoses.${detail.value.diagnosis_code}.${field}`;
  return te(key) ? t(key) : detail.value[`diagnosis_${field}`];
}

const loading = ref(false);
const loadError = ref(false);
const detail = ref<UserErrorRequestDetail | null>(null);
let requestVersion = 0;

watch(
  () => [props.show, props.errorId] as const,
  ([show, id], _, onCleanup) => {
    onCleanup(() => {
      requestVersion++;
    });
    if (show && id != null) {
      fetchDetail(id);
    } else if (!show) {
      detail.value = null;
      loadError.value = false;
    }
  },
);

async function fetchDetail(id: number) {
  const version = ++requestVersion;
  loading.value = true;
  loadError.value = false;
  detail.value = null;
  try {
    const result = await getMyErrorDetail(id);
    if (version === requestVersion) detail.value = result;
  } catch (e) {
    if (version !== requestVersion) return;
    console.error("[UserErrorDetailModal] Failed to load error detail:", e);
    loadError.value = true;
  } finally {
    if (version === requestVersion) loading.value = false;
  }
}

function statusClass(code: number) {
  if (code >= 500) return "badge-danger";
  if (code === 429) return "badge-warning";
  return "badge-gray";
}
</script>
