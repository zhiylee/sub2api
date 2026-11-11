//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/distribution"
	"github.com/stretchr/testify/require"
)

func TestUpdateServiceUpstreamDoesNotOfferInstallation(t *testing.T) {
	cache := &updateServiceCacheStub{}
	client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{
		distribution.Repository: {TagName: "v0.2.21-zy.1"},
		distribution.UpstreamRepository: {
			TagName: "v0.2.22", Name: "Official 0.2.22", PublishedAt: "2026-11-12T00:00:00Z",
			HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.22",
			Assets:  []GitHubAsset{{Name: "official binary", BrowserDownloadURL: "https://github.com/official"}},
		},
	}}
	svc := NewUpdateService(cache, client, "0.2.21-zy.1", "release")
	info := svc.CheckUpstreamRelease(context.Background(), false)
	require.Equal(t, "0.2.21", info.BaseVersion)
	require.Equal(t, "0.2.22", info.LatestVersion)
	require.True(t, info.HasNewRelease)
	require.Empty(t, info.Warning)
	require.Empty(t, cache.data)
	data, err := json.Marshal(info)
	require.NoError(t, err)
	require.NotContains(t, string(data), "assets")
	require.NotContains(t, string(data), "download")

	// An official update neither changes the own update flag nor triggers an official fetch on installation.
	err = svc.PerformUpdate(context.Background())
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
	require.Equal(t, []string{distribution.UpstreamRepository, distribution.Repository}, client.latestRepos)
}

func TestUpdateServiceUpstreamCacheAndForcedRefresh(t *testing.T) {
	client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{
		distribution.UpstreamRepository: {TagName: "v0.2.22"},
	}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-zy.10", "release")
	info := svc.CheckUpstreamRelease(context.Background(), false)
	require.True(t, info.HasNewRelease)
	require.Equal(t, distribution.UpstreamReleasesURL, info.HTMLURL)
	svc.currentVersion = "0.2.22-zy.1"
	info = svc.CheckUpstreamRelease(context.Background(), false)
	require.True(t, info.Cached)
	require.Equal(t, "0.2.22", info.BaseVersion)
	require.False(t, info.HasNewRelease)
	require.Len(t, client.latestRepos, 1)
	client.latest[distribution.UpstreamRepository] = &GitHubRelease{TagName: "v0.2.23"}
	info = svc.CheckUpstreamRelease(context.Background(), true)
	require.Equal(t, "0.2.23", info.LatestVersion)
	require.True(t, info.HasNewRelease)
	require.False(t, info.Cached)
}

func TestUpdateServiceUpstreamFailureDoesNotBlockOwnChannel(t *testing.T) {
	client := &releaseChannelClientStub{
		latest:    map[string]*GitHubRelease{distribution.Repository: {TagName: "v0.2.21-zy.2"}},
		latestErr: map[string]error{distribution.UpstreamRepository: errors.New("official offline")},
	}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-zy.1", "release")
	upstream := svc.CheckUpstreamRelease(context.Background(), false)
	require.Contains(t, upstream.Warning, "official offline")
	require.Empty(t, upstream.LatestVersion)
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	require.Empty(t, info.Warning)
	// Own assets are selected; the missing archive fails before touching the binary.
	require.ErrorContains(t, svc.PerformUpdate(context.Background()), "no compatible release")
	require.Equal(t, []string{distribution.UpstreamRepository, distribution.Repository, distribution.Repository}, client.latestRepos)
}

func TestUpdateServiceUpstreamCacheFallbackAndValidation(t *testing.T) {
	for name, mutate := range map[string]func(*upstreamReleaseCacheEntry){
		"valid":              func(*upstreamReleaseCacheEntry) {},
		"expired":            func(e *upstreamReleaseCacheEntry) { e.Timestamp -= updateCacheTTL + 1 },
		"wrong source":       func(e *upstreamReleaseCacheEntry) { e.Repository = distribution.Repository },
		"downstream version": func(e *upstreamReleaseCacheEntry) { e.Info.LatestVersion = "0.2.22-zy.1" },
	} {
		t.Run(name, func(t *testing.T) {
			entry := upstreamReleaseCacheEntry{
				Repository: distribution.UpstreamRepository, Timestamp: time.Now().Unix(),
				Info: UpstreamReleaseInfo{LatestVersion: "0.2.22", HTMLURL: distribution.UpstreamReleasesURL},
			}
			mutate(&entry)
			data, err := json.Marshal(entry)
			require.NoError(t, err)
			client := &releaseChannelClientStub{latestErr: map[string]error{distribution.UpstreamRepository: errors.New("offline")}}
			svc := NewUpdateService(&updateServiceCacheStub{upstreamData: string(data)}, client, "0.2.21-zy.1", "release")
			info := svc.CheckUpstreamRelease(context.Background(), true)
			require.Contains(t, info.Warning, "offline")
			require.Equal(t, name == "valid", info.Cached)
			require.Equal(t, name == "valid", info.HasNewRelease)
		})
	}
}

func TestUpdateServiceUpstreamRejectsNonStableReleases(t *testing.T) {
	for _, release := range []*GitHubRelease{
		nil, {TagName: "v0.2.22-rc.1"}, {TagName: "v0.2.22-zy.1"},
		{TagName: "v0.2.22", Draft: true}, {TagName: "v0.2.22", Prerelease: true},
	} {
		client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{distribution.UpstreamRepository: release}}
		svc := NewUpdateService(&updateServiceCacheStub{}, client, "dev", "source")
		info := svc.CheckUpstreamRelease(context.Background(), false)
		require.NotEmpty(t, info.Warning)
		require.Empty(t, info.BaseVersion)
		require.False(t, info.HasNewRelease)
	}
	client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{distribution.UpstreamRepository: {TagName: "v0.2.22"}}}
	info := NewUpdateService(&updateServiceCacheStub{}, client, "dev", "source").CheckUpstreamRelease(context.Background(), false)
	require.Empty(t, info.BaseVersion)
	require.Equal(t, "0.2.22", info.LatestVersion)
	require.False(t, info.HasNewRelease)
}
