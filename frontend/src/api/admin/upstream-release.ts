import { apiClient } from '../client'

/** Official release metadata is display-only and contains no install assets. */
export interface UpstreamReleaseInfo {
  base_version: string
  latest_version: string
  has_new_release: boolean
  name: string
  published_at: string
  html_url: string
  cached: boolean
  warning?: string
}

export async function checkUpstreamRelease(force = false): Promise<UpstreamReleaseInfo> {
  const { data } = await apiClient.get<UpstreamReleaseInfo>('/admin/system/upstream-release', {
    params: force ? { force: 'true' } : undefined
  })
  return data
}
