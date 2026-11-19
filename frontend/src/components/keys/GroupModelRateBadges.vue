<template>
  <div
    v-if="rates.length"
    class="mt-2 flex w-full flex-wrap items-start gap-1.5 text-left"
    :aria-label="t('keys.modelRates')"
    role="group"
  >
    <span
      v-for="{ model, multiplier } in rates"
      :key="model"
      class="inline-flex max-w-full flex-wrap items-center gap-x-1.5 rounded-md px-1.5 py-0.5 text-xs leading-normal"
      :class="multiplier < groupRate
        ? 'bg-amber-100 font-medium text-amber-900 dark:bg-amber-900/30 dark:text-amber-300'
        : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'"
    >
      <span v-if="multiplier < groupRate" class="inline-flex items-center gap-1">
        <Icon name="gift" size="xs" aria-hidden="true" />
        {{ t('keys.modelRateOffer') }}
      </span>
      <span class="min-w-0 [overflow-wrap:anywhere]">{{ model }}</span>
      <span class="whitespace-nowrap font-semibold tabular-nums">{{ multiplier }}×</span>
    </span>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { GroupModelRate } from '@/types'

defineProps<{ rates: GroupModelRate[], groupRate: number }>()
const { t } = useI18n()
</script>
