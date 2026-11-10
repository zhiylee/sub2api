<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'

const force = defineModel<boolean>('force', { default: false })
const free = defineModel<boolean>('free', { default: false })
const { t } = useI18n()
</script>

<template>
  <div class="space-y-4 border-t border-gray-200 pt-4 dark:border-dark-600">
    <div class="input-label">{{ t('admin.accounts.openaiFast.title') }}</div>
    <div class="flex items-center justify-between gap-4">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.openaiFast.force') }}</label>
        <p class="input-hint">{{ t('admin.accounts.openaiFast.forceHint') }}</p>
      </div>
      <Toggle
        v-model="force"
        data-testid="openai-account-force-fast-toggle"
        :aria-label="t('admin.accounts.openaiFast.force')"
      />
    </div>
    <div class="flex items-center justify-between gap-4">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.openaiFast.free') }}</label>
        <p class="input-hint">{{ t('admin.accounts.openaiFast.freeHint') }}</p>
      </div>
      <Toggle
        v-model="free"
        data-testid="openai-account-free-fast-toggle"
        :aria-label="t('admin.accounts.openaiFast.free')"
      />
    </div>
    <p class="input-hint">{{ t('admin.accounts.openaiFast.inheritHint') }}</p>
  </div>
</template>
