<template>
  <span class="inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-semibold">
    <span :class="labelClass">{{ getUsageServiceTierLabel(tier, t) }}</span>
    <span
      v-if="free"
      class="inline-flex rounded px-1 py-px text-[10px] leading-tight ring-1 ring-inset"
      :class="onDarkSurface
        ? 'bg-emerald-400/10 text-emerald-200 ring-emerald-400/30'
        : 'bg-emerald-50 text-emerald-800 ring-emerald-200 dark:bg-emerald-400/10 dark:text-emerald-200 dark:ring-emerald-400/30'"
    >
      <span aria-hidden="true">Free</span>
      <span class="sr-only">{{ t('usage.freeAcceleration') }}</span>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatUsageServiceTier, getUsageServiceTierLabel } from '@/utils/usageServiceTier'

const props = defineProps<{
  tier?: string | null
  free?: boolean
  onDarkSurface?: boolean
}>()

const { t } = useI18n()
const gradients = {
  priority: {
    themed: 'from-cyan-700 to-blue-700 dark:from-cyan-300 dark:to-blue-400',
    dark: 'from-cyan-300 to-blue-400',
  },
  ultrafast: {
    themed: 'from-violet-700 to-pink-700 dark:from-violet-300 dark:to-pink-300',
    dark: 'from-violet-300 to-pink-300',
  },
}

const labelClass = computed(() => {
  const tier = formatUsageServiceTier(props.tier)
  if (tier === 'priority' || tier === 'ultrafast') {
    const gradient = gradients[tier]
    return [
      'bg-gradient-to-r bg-clip-text text-transparent forced-colors:text-[CanvasText]',
      props.onDarkSurface ? gradient.dark : gradient.themed,
    ]
  }
  return props.onDarkSurface ? 'text-cyan-300' : 'text-cyan-700 dark:text-cyan-300'
})
</script>
