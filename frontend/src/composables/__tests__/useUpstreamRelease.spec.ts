import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useUpstreamRelease } from '../useUpstreamRelease'
import { checkUpstreamRelease, type UpstreamReleaseInfo } from '@/api/admin/upstream-release'

vi.mock('@/api/admin/upstream-release', () => ({ checkUpstreamRelease: vi.fn() }))

const release: UpstreamReleaseInfo = {
  base_version: '0.2.21', latest_version: '0.2.22', has_new_release: true,
  name: 'Official 0.2.22', published_at: '2026-11-12T00:00:00Z',
  html_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.22', cached: false
}

beforeEach(() => vi.mocked(checkUpstreamRelease).mockReset())
afterEach(() => vi.useRealTimers())

describe('independent official release checks', () => {
  it('deduplicates concurrent checks, caches results, and forces refresh', async () => {
    vi.mocked(checkUpstreamRelease).mockResolvedValue(release)
    const state = useUpstreamRelease()
    const first = state.refresh()
    const second = state.refresh()
    expect(second).toBe(first)
    expect(state.loading.value).toBe(true)
    await first
    expect(state.hasNewRelease.value).toBe(true)
    expect(state.loading.value).toBe(false)
    await state.refresh()
    expect(checkUpstreamRelease).toHaveBeenCalledOnce()
    await state.refresh(true)
    expect(checkUpstreamRelease).toHaveBeenLastCalledWith(true)
    expect(checkUpstreamRelease).toHaveBeenCalledTimes(2)
  })

  it('refreshes the local cache after its lifetime', async () => {
    vi.useFakeTimers()
    vi.mocked(checkUpstreamRelease).mockResolvedValue(release)
    const state = useUpstreamRelease()
    await state.refresh()
    vi.advanceTimersByTime(20 * 60 * 1000)
    await state.refresh()
    expect(checkUpstreamRelease).toHaveBeenCalledTimes(2)
  })

  it('keeps the last result on failure and retries without poisoning other instances', async () => {
    vi.mocked(checkUpstreamRelease).mockResolvedValueOnce(release).mockRejectedValueOnce(new Error('offline'))
    const state = useUpstreamRelease()
    await state.refresh()
    expect(await state.refresh(true)).toBeNull()
    expect(state.error.value).toBe(true)
    expect(state.info.value?.latest_version).toBe('0.2.22')
    expect(state.loading.value).toBe(false)
    expect(useUpstreamRelease().info.value).toBeNull()
    vi.mocked(checkUpstreamRelease).mockResolvedValue({ ...release, has_new_release: false })
    await state.refresh()
    expect(state.error.value).toBe(false)
    expect(state.hasNewRelease.value).toBe(false)
  })

  it('reports server cache warnings independently of the update store', async () => {
    vi.mocked(checkUpstreamRelease).mockResolvedValue({ ...release, cached: true, warning: 'Using cached data' })
    const state = useUpstreamRelease()
    await state.refresh()
    expect(state.error.value).toBe(true)
    expect(state.hasNewRelease.value).toBe(true)
  })
})
