<template>
  <div v-if="outbound" class="flex min-w-0 flex-col gap-0.5 text-xs" data-testid="account-cpr-outbound">
    <div class="flex min-w-0 flex-wrap items-baseline gap-x-1">
      <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.cprOutbound') }}</span>
      <span class="break-all font-mono text-gray-700 dark:text-gray-300" data-testid="cpr-outbound-value">{{ value }}</span>
    </div>
    <time
      v-if="outbound.observedAt"
      :datetime="outbound.observedAt"
      class="text-gray-400 dark:text-gray-500"
    >{{ t('admin.accounts.cprOutboundObservedAt', { time: formatDateTimeToMinute(outbound.observedAt) }) }}</time>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { cprOutboundProxy, type CPRProxyAccount } from '@/utils/cprOutboundProxy'
import { formatDateTimeToMinute } from '@/utils/format'

const props = defineProps<{ account: CPRProxyAccount | null }>()
const { t } = useI18n()
const outbound = computed(() => cprOutboundProxy(props.account))
const value = computed(() => outbound.value?.status === 'proxy'
  ? outbound.value.endpoint
  : t(outbound.value?.status === 'direct' ? 'admin.accounts.cprOutboundDirect' : 'admin.accounts.cprOutboundUnknown'))
</script>
