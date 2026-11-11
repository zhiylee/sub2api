<template>
  <section
    class="mt-4 space-y-2 border-t border-gray-100 pt-3 text-xs dark:border-dark-700"
    :aria-label="t('downstreamUpdates.upstream.title')"
  >
    <p class="font-medium text-gray-600 dark:text-dark-300">
      {{ t('downstreamUpdates.upstream.title') }}
    </p>
    <p v-if="loading" class="text-gray-400 dark:text-dark-500">{{ t('common.loading') }}</p>
    <template v-else>
      <p class="break-words text-gray-500 dark:text-dark-400">
        {{ info?.base_version
          ? `${t('downstreamUpdates.upstream.baseVersion')}: v${info.base_version}`
          : t('downstreamUpdates.upstream.unknownBase') }}
      </p>
      <template v-if="info?.latest_version">
        <p class="break-words font-medium text-gray-900 dark:text-white">
          {{ t('downstreamUpdates.upstream.latestVersion') }}: v{{ info.latest_version }}
        </p>
        <p v-if="publishedDate" class="text-gray-400 dark:text-dark-500">
          {{ t('downstreamUpdates.upstream.releasedAt', { date: publishedDate }) }}
        </p>
        <p v-if="info.has_new_release" class="text-blue-600 dark:text-blue-400">
          {{ t('downstreamUpdates.upstream.pendingIntegration') }}
        </p>
      </template>
      <p v-if="error" class="text-amber-600 dark:text-amber-400">
        {{ info?.latest_version
          ? t('downstreamUpdates.upstream.cachedWarning')
          : t('downstreamUpdates.upstream.checkFailed') }}
      </p>
    </template>
    <a
      :href="info?.html_url || UPSTREAM_RELEASES_URL"
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-1 text-primary-600 hover:underline dark:text-primary-400"
    >
      {{ t('downstreamUpdates.upstream.viewRelease') }}
      <Icon name="externalLink" size="xs" :stroke-width="2" />
    </a>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UpstreamReleaseInfo } from '@/api/admin/upstream-release'
import { UPSTREAM_RELEASES_URL } from '@/config/distribution'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  info: UpstreamReleaseInfo | null
  loading: boolean
  error: boolean
}>()
const { t } = useI18n()
const publishedDate = computed(() => {
  if (!props.info?.published_at) return ''
  const date = new Date(props.info.published_at)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString()
})
</script>
