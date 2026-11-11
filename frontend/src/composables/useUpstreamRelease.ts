import { computed, readonly, ref } from 'vue'
import { checkUpstreamRelease, type UpstreamReleaseInfo } from '@/api/admin/upstream-release'

const CACHE_TTL_MS = 20 * 60 * 1000

// Local to the admin version component; official checks never enter the install store.
export function useUpstreamRelease() {
  const info = ref<UpstreamReleaseInfo | null>(null)
  const loading = ref(false)
  const error = ref(false)
  let fetchedAt = 0
  let pending: Promise<UpstreamReleaseInfo | null> | null = null

  function refresh(force = false): Promise<UpstreamReleaseInfo | null> {
    if (pending) return pending
    if (!force && info.value && !error.value && Date.now() - fetchedAt < CACHE_TTL_MS) {
      return Promise.resolve(info.value)
    }
    loading.value = true
    pending = Promise.resolve()
      .then(() => checkUpstreamRelease(force))
      .then((data) => {
        info.value = data
        error.value = !!data.warning
        fetchedAt = Date.now()
        return data
      })
      .catch(() => {
        error.value = true
        return null
      })
      .finally(() => {
        loading.value = false
        pending = null
      })
    return pending
  }

  return {
    info: readonly(info),
    loading: readonly(loading),
    error: readonly(error),
    hasNewRelease: computed(() => !!info.value?.has_new_release),
    refresh
  }
}
