<template>
  <BaseDialog
    :show="show"
    :title="
      t('admin.accounts.sessionOccupants.title', { name: account?.name ?? '' })
    "
    width="normal"
    @close="emit('close')"
  >
    <div class="min-h-32">
      <div v-if="loading" class="py-10 text-center text-sm text-gray-500">
        {{ t("admin.accounts.sessionOccupants.loading") }}
      </div>
      <div v-else-if="error" class="py-8 text-center">
        <p class="mb-3 text-sm text-red-600">
          {{ t("admin.accounts.sessionOccupants.error") }}
        </p>
        <button
          data-testid="session-occupants-refresh"
          class="btn btn-secondary"
          @click="load"
        >
          {{ t("admin.accounts.sessionOccupants.refresh") }}
        </button>
      </div>
      <div
        v-else-if="!data?.occupants.length"
        class="py-10 text-center text-sm text-gray-500"
      >
        {{ t("admin.accounts.sessionOccupants.empty") }}
      </div>
      <div v-else class="space-y-2">
        <div
          v-for="(occupant, index) in data.occupants"
          :key="occupant.user_id ?? `unknown-${index}`"
          class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-600"
        >
          <div class="min-w-0">
            <p
              class="truncate text-sm font-medium text-gray-900 dark:text-white"
            >
              {{
                occupant.username ||
                occupant.email ||
                t("admin.accounts.sessionOccupants.unknown")
              }}
            </p>
            <p class="truncate text-xs text-gray-500">
              <template v-if="occupant.user_id != null"
                >#{{ occupant.user_id
                }}<template v-if="occupant.email">
                  · {{ occupant.email }}</template
                ></template
              >
              <template v-else>{{
                t("admin.accounts.sessionOccupants.legacyHint")
              }}</template>
            </p>
            <p class="mt-1 text-xs text-gray-400">
              {{ t("admin.accounts.sessionOccupants.lastActive") }}:
              {{ formatDateTime(occupant.last_active) }}
            </p>
          </div>
          <span
            class="ml-3 rounded-full bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
            >{{ occupant.session_count }}
            {{ t("admin.accounts.sessionOccupants.sessions") }}</span
          >
        </div>
        <button
          data-testid="session-occupants-refresh"
          class="btn btn-secondary mt-3"
          @click="load"
        >
          {{ t("admin.accounts.sessionOccupants.refresh") }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import BaseDialog from "@/components/common/BaseDialog.vue";
import { adminAPI } from "@/api/admin";
import { formatDateTime } from "@/utils/format";
import type { Account, AccountSessionOccupancy } from "@/types";

const props = defineProps<{
  show: boolean;
  account: Pick<Account, "id" | "name"> | null;
}>();
const emit = defineEmits<{ (event: "close"): void }>();
const { t } = useI18n();
const loading = ref(false);
const error = ref(false);
const data = ref<AccountSessionOccupancy | null>(null);
let requestSequence = 0;

async function load() {
  if (!props.account) return;
  const sequence = ++requestSequence;
  const accountID = props.account.id;
  loading.value = true;
  error.value = false;
  try {
    const response = await adminAPI.accounts.getSessionOccupants(accountID);
    if (
      sequence !== requestSequence ||
      !props.show ||
      props.account?.id !== accountID
    )
      return;
    data.value = response;
  } catch {
    if (
      sequence !== requestSequence ||
      !props.show ||
      props.account?.id !== accountID
    )
      return;
    error.value = true;
  } finally {
    if (sequence === requestSequence) loading.value = false;
  }
}

watch(
  () => [props.show, props.account?.id] as const,
  ([show]) => {
    if (show) void load();
    else {
      requestSequence++;
      data.value = null;
      loading.value = false;
    }
  },
  { immediate: true },
);
</script>
