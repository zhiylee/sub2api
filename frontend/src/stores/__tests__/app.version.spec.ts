import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '../app'
import { checkUpdates, type VersionInfo } from '@/api/admin/system'

vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn() }))
vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))

const version: VersionInfo = {
  current_version: '0.2.21-zy.1', latest_version: '0.2.21-zy.2',
  has_update: true, cached: true, build_type: 'release'
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.mocked(checkUpdates).mockReset()
})

describe('own release query status', () => {
  it('preserves server warnings in the frontend cache and clears them after recovery', async () => {
    vi.mocked(checkUpdates).mockResolvedValueOnce({ ...version, warning: 'Using cached data' }).mockResolvedValueOnce(version)
    const store = useAppStore()
    await store.fetchVersion()
    expect(store.versionWarning).toBe('Using cached data')
    expect((await store.fetchVersion())?.warning).toBe('Using cached data')
    expect(checkUpdates).toHaveBeenCalledOnce()
    await store.fetchVersion(true)
    expect(store.versionWarning).toBe('')
    expect(store.hasUpdate).toBe(true)
  })

  it('marks request failure instead of presenting an unverified up-to-date state', async () => {
    vi.mocked(checkUpdates).mockRejectedValue(new Error('offline'))
    const log = vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      const store = useAppStore()
      expect(await store.fetchVersion()).toBeNull()
      expect(store.versionWarning).not.toBe('')
      expect(store.versionLoading).toBe(false)
      expect(store.versionLoaded).toBe(false)
    } finally {
      log.mockRestore()
    }
  })
})
