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

type releaseChannelClientStub struct {
	latest      map[string]*GitHubRelease
	latestErr   map[string]error
	recent      []*GitHubRelease
	latestRepos []string
	recentRepos []string
}

func (c *releaseChannelClientStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	c.latestRepos = append(c.latestRepos, repo)
	return c.latest[repo], c.latestErr[repo]
}

func (c *releaseChannelClientStub) FetchRecentReleases(_ context.Context, repo string, _ int) ([]*GitHubRelease, error) {
	c.recentRepos = append(c.recentRepos, repo)
	return c.recent, nil
}

func (*releaseChannelClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("test must not download or replace the running binary")
}

func (*releaseChannelClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("test must not download checksums")
}

func (s *updateServiceCacheStub) GetUpstreamReleaseInfo(context.Context) (string, error) {
	if s.upstreamData == "" {
		return "", errors.New("cache miss")
	}
	return s.upstreamData, nil
}

func (s *updateServiceCacheStub) SetUpstreamReleaseInfo(_ context.Context, data string, _ time.Duration) error {
	s.upstreamData = data
	return nil
}

func TestUpdateServiceDownstreamRevisionAndCache(t *testing.T) {
	cache := &updateServiceCacheStub{}
	client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{
		distribution.Repository: {TagName: "v0.2.21-zy.10"},
	}}
	svc := NewUpdateService(cache, client, "0.2.21-zy.2", "release")
	info, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	require.Empty(t, info.Warning)
	require.Equal(t, []string{distribution.Repository}, client.latestRepos)
	require.Contains(t, cache.data, `"repository":"zhiylee/sub2api"`)
	require.Empty(t, cache.upstreamData)

	// Redis survives an upgrade; comparison must use the new running version.
	svc.currentVersion = "0.2.21-zy.10"
	info, err = svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.False(t, info.HasUpdate)
	require.Len(t, client.latestRepos, 1)
}

func TestUpdateServiceDownstreamRejectsOtherReleases(t *testing.T) {
	for name, release := range map[string]*GitHubRelease{
		"missing":    nil,
		"official":   {TagName: "v0.2.22"},
		"rc":         {TagName: "v0.2.22-rc.1"},
		"draft":      {TagName: "v0.2.22-zy.1", Draft: true},
		"prerelease": {TagName: "v0.2.22-zy.1", Prerelease: true},
		"invalid":    {TagName: "v0.2.22-zy.01"},
	} {
		t.Run(name, func(t *testing.T) {
			client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{distribution.Repository: release}}
			svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-zy.1", "release")
			info, err := svc.CheckUpdate(context.Background(), true)
			require.NoError(t, err)
			require.False(t, info.HasUpdate)
			require.NotEmpty(t, info.Warning)
			err = svc.PerformUpdate(context.Background())
			require.ErrorContains(t, err, "update check failed")
			require.NotErrorIs(t, err, ErrNoUpdateAvailable)
			require.Equal(t, []string{distribution.Repository, distribution.Repository}, client.latestRepos)
		})
	}
}

func TestUpdateServiceDownstreamOldCacheCannotSelectOfficialAssets(t *testing.T) {
	for _, cached := range []struct{ name, repository, latest string }{
		{"legacy identity", "", "0.2.22-zy.1"},
		{"wrong repository", distribution.UpstreamRepository, "0.2.22-zy.1"},
		{"official version", distribution.Repository, "0.2.22"},
	} {
		t.Run(cached.name, func(t *testing.T) {
			data, err := json.Marshal(map[string]any{
				"repository": cached.repository, "latest": cached.latest,
				"release_info": &ReleaseInfo{HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.22"},
				"timestamp":    time.Now().Unix(),
			})
			require.NoError(t, err)
			client := &releaseChannelClientStub{}
			svc := NewUpdateService(&updateServiceCacheStub{data: string(data)}, client, "0.2.21-zy.1", "release")
			info, err := svc.CheckUpdate(context.Background(), false)
			require.NoError(t, err)
			require.False(t, info.Cached)
			require.False(t, info.HasUpdate)
			require.Nil(t, info.ReleaseInfo)
			require.Equal(t, []string{distribution.Repository}, client.latestRepos)
		})
	}
}

func TestUpdateServiceDownstreamFailedRefreshCannotInstallCachedUpdate(t *testing.T) {
	client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{
		distribution.Repository: {TagName: "v0.2.21-zy.2"},
	}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-zy.1", "release")
	_, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	client.latestErr = map[string]error{distribution.Repository: errors.New("offline")}
	err = svc.PerformUpdate(context.Background())
	require.ErrorContains(t, err, "offline")
	require.NotErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestUpdateServiceDownstreamRollbackUsesOnlyOwnRevisions(t *testing.T) {
	client := &releaseChannelClientStub{recent: []*GitHubRelease{
		{TagName: "v0.2.21-zy.2"}, {TagName: "v0.2.21-zy.9"}, {TagName: "v0.2.21-zy.10"},
		{TagName: "v0.2.21-zy.11"}, {TagName: "v0.2.21-zy.1"}, {TagName: "v0.2.20-zy.1"},
		{TagName: "v0.2.20"}, {TagName: "v0.2.21-zy.8", Draft: true},
		{TagName: "v0.2.21-zy.7", Prerelease: true}, {TagName: "v0.2.21-rc.1"}, nil,
	}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-zy.10", "release")
	versions, err := svc.ListRollbackVersions(context.Background())
	require.NoError(t, err)
	require.Equal(t, []RollbackVersion{
		{Version: "0.2.21-zy.9"}, {Version: "0.2.21-zy.2"}, {Version: "0.2.21-zy.1"},
	}, versions)
	require.Equal(t, []string{distribution.Repository}, client.recentRepos)
	require.ErrorIs(t, svc.RollbackToVersion(context.Background(), "0.2.20"), ErrRollbackVersionNotAllowed)
	// Missing assets stop before download, after the downstream allowlist succeeds.
	require.ErrorContains(t, svc.RollbackToVersion(context.Background(), "v0.2.21-zy.9"), "no compatible release")
	require.Empty(t, client.latestRepos)
}

func TestUpdateServiceDownstreamUnknownBuildIsNotInstallable(t *testing.T) {
	client := &releaseChannelClientStub{latest: map[string]*GitHubRelease{
		distribution.Repository: {TagName: "v0.2.21-zy.1"},
	}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "dev", "source")
	info, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.False(t, info.HasUpdate)
	_, err = svc.ListRollbackVersions(context.Background())
	require.ErrorContains(t, err, "cannot determine current release version")
}
