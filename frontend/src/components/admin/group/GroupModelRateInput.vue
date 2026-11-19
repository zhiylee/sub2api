<template>
  <label class="mt-3 block max-w-xs">
    <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.modelPricing.rate') }}</span>
    <input
      :value="modelValue"
      type="number"
      step="any"
      min="0"
      class="input mt-1 text-sm"
      :placeholder="t('admin.groups.modelPricing.rateFallback')"
      :aria-invalid="!isValidPositiveMultiplier(modelValue)"
      @input="update(($event.target as HTMLInputElement).value)"
    />
  </label>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { isValidPositiveMultiplier } from '@/components/admin/channel/types'

defineProps<{ modelValue?: number | string | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: string | null] }>()
const { t } = useI18n()

function update(value: string) {
  emit('update:modelValue', value === '' ? null : value)
}
</script>
