<template>
  <template v-if="amounts">
    <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
      <span class="text-gray-400">{{ t('usage.beforeDiscountCharge') }}</span>
      <span class="font-medium tabular-nums text-white">${{ amounts.before }}</span>
    </div>
    <div class="flex items-center justify-between gap-6">
      <span class="text-gray-400">{{ t('usage.freeTierDiscount', { tier: getUsageServiceTierLabel(usage.service_tier, t) }) }}</span>
      <span class="font-medium tabular-nums text-emerald-300">−${{ amounts.discount }}</span>
    </div>
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import type { UsageLog } from '@/types'

const props = defineProps<{
  usage: Pick<UsageLog, 'service_tier' | 'actual_cost' | 'actual_cost_before_discount'>
}>()
const { t } = useI18n()

const amounts = computed(() => {
  const { actual_cost: charged, actual_cost_before_discount: before } = props.usage
  if (before == null || !Number.isFinite(before) || !Number.isFinite(charged) || charged < 0 || before <= charged) {
    return null
  }

  const precision = 8
  const beforeAmount = before.toFixed(precision)
  // Subtract the displayed amounts so the three rows reconcile at the shown precision.
  const beforeUnits = BigInt(beforeAmount.replace('.', ''))
  const chargedUnits = BigInt(charged.toFixed(precision).replace('.', ''))
  const discountUnits = beforeUnits - chargedUnits
  if (discountUnits === 0n) return null

  const digits = discountUnits.toString().padStart(precision + 1, '0')
  return { before: beforeAmount, discount: `${digits.slice(0, -precision)}.${digits.slice(-precision)}` }
})
</script>
