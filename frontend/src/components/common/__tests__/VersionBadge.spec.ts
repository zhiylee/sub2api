import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import VersionBadge from '../VersionBadge.vue'
import type { UpstreamReleaseInfo } from '@/api/admin/upstream-release'

const mocks = vi.hoisted(() => ({
  auth: { isAdmin: true },
  store: {
    versionLoading: false,
    currentVersion: '0.2.21-zy.2',
    latestVersion: '0.2.21-zy.2',
    hasUpdate: false,
    versionWarning: '',
    buildType: 'release',
    releaseInfo: { html_url: 'https://github.com/zhiylee/sub2api/releases/tag/v0.2.21-zy.2' },
    fetchVersion: vi.fn().mockResolvedValue(null),
    clearVersionCache: vi.fn()
  },
  official: vi.fn(),
  performUpdate: vi.fn(),
  rollback: vi.fn(),
  getRollbackVersions: vi.fn(),
  restartService: vi.fn()
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => mocks.auth,
  useAppStore: () => reactive(mocks.store)
}))
vi.mock('@/api/admin/system', () => ({
  performUpdate: mocks.performUpdate,
  rollback: mocks.rollback,
  getRollbackVersions: mocks.getRollbackVersions,
  restartService: mocks.restartService
}))
vi.mock('@/api/admin/upstream-release', () => ({ checkUpstreamRelease: mocks.official }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() })
}))

const official: UpstreamReleaseInfo = {
  base_version: '0.2.21', latest_version: '0.2.22', has_new_release: true,
  name: 'Official 0.2.22', published_at: '2026-11-12T00:00:00Z',
  html_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.22', cached: false
}

enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.clearAllMocks()
  mocks.auth.isAdmin = true
  mocks.store.hasUpdate = false
  mocks.store.versionWarning = ''
  mocks.store.versionLoading = false
  mocks.store.buildType = 'release'
  mocks.official.mockReset().mockResolvedValue(official)
  mocks.performUpdate.mockResolvedValue({ need_restart: true })
  mocks.getRollbackVersions.mockResolvedValue({ versions: [{
    version: '0.2.21-zy.1', published_at: '',
    html_url: 'https://github.com/zhiylee/sub2api/releases/tag/v0.2.21-zy.1'
  }] })
})

function mountBadge() {
  return mount(VersionBadge, { global: { stubs: { Icon: true } } })
}

describe('fork updates and official releases', () => {
  it('announces an official release outside the dropdown without offering installation', async () => {
    const wrapper = mountBadge()
    await flushPromises()
    expect(wrapper.get('button').text()).toContain('downstreamUpdates.upstream.newRelease')
    await wrapper.get('button').trigger('click')
    const notice = wrapper.get('section')
    expect(notice.text()).toContain('v0.2.22')
    expect(notice.text()).toContain('v0.2.21')
    expect(notice.get('a').attributes('href')).toBe(official.html_url)
    expect(notice.get('a').attributes('rel')).toBe('noopener noreferrer')
    expect(notice.find('button').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('version.updateNow')
    expect(wrapper.text()).toContain('downstreamUpdates.upToDate')
    expect(mocks.performUpdate).not.toHaveBeenCalled()
  })

  it('preserves the own install action while official information is unavailable', async () => {
    mocks.store.hasUpdate = true
    mocks.official.mockRejectedValue(new Error('official offline'))
    const wrapper = mountBadge()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    expect(wrapper.get('section').text()).toContain('downstreamUpdates.upstream.checkFailed')
    const button = wrapper.findAll('button').find(b => b.text() === 'version.updateNow')!
    expect(button.attributes('disabled')).toBeUndefined()
    await button.trigger('click')
    await flushPromises()
    expect(mocks.performUpdate).toHaveBeenCalledOnce()
    expect(mocks.store.clearVersionCache).toHaveBeenCalledOnce()
  })

  it('refreshes both channels and keeps cached official metadata clearly marked', async () => {
    mocks.official.mockResolvedValue({ ...official, cached: true, warning: 'offline' })
    const wrapper = mountBadge()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    expect(wrapper.get('section').text()).toContain('downstreamUpdates.upstream.cachedWarning')
    await wrapper.get('button[title="version.refresh"]').trigger('click')
    await flushPromises()
    expect(mocks.store.fetchVersion).toHaveBeenLastCalledWith(true)
    expect(mocks.official).toHaveBeenLastCalledWith(true)
  })

  it('does not label a failed own check as current or allow installation from stale metadata', async () => {
    mocks.store.hasUpdate = true
    mocks.store.versionWarning = 'Using cached data: own channel offline'
    const wrapper = mountBadge()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('downstreamUpdates.checkFailed')
    expect(wrapper.text()).not.toContain('downstreamUpdates.upToDate')
    const button = wrapper.findAll('button').find(b => b.text() === 'version.updateNow')!
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.get('section').text()).toContain('v0.2.22')
  })

  it('uses the fork script and GHCR image in manual rollback commands', async () => {
    const wrapper = mountBadge()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'version.rollback')!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('v0.2.21-zy.1'))!.trigger('click')
    expect(wrapper.text()).toContain('https://raw.githubusercontent.com/zhiylee/sub2api/v0.2.21-zy.2/deploy/install.sh')
    expect(wrapper.text()).toContain('rollback v0.2.21-zy.1')
    await wrapper.findAll('button').find(b => b.text() === 'version.deployDocker')!.trigger('click')
    expect(wrapper.text()).toContain('image: ghcr.io/zhiylee/sub2api:0.2.21-zy.1')
    expect(wrapper.text()).not.toContain('image: weishaw/sub2api')
    expect(wrapper.text()).toContain('downstreamUpdates.dockerHint')
  })

  it('keeps source builds read-only and shows an unknown official base honestly', async () => {
    mocks.store.buildType = 'source'
    mocks.store.hasUpdate = true
    mocks.official.mockResolvedValue({ ...official, base_version: '', has_new_release: false })
    const wrapper = mountBadge()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('version.sourceModeHint')
    expect(wrapper.text()).not.toContain('version.updateNow')
    expect(wrapper.get('section').text()).toContain('downstreamUpdates.upstream.unknownBase')
  })

  it('does not query either release channel for non-admin users', async () => {
    mocks.auth.isAdmin = false
    const wrapper = mount(VersionBadge, { props: { version: '0.2.21-zy.2' } })
    await flushPromises()
    expect(wrapper.text()).toBe('v0.2.21-zy.2')
    expect(wrapper.find('section').exists()).toBe(false)
    expect(mocks.official).not.toHaveBeenCalled()
    expect(mocks.store.fetchVersion).not.toHaveBeenCalled()
  })
})
