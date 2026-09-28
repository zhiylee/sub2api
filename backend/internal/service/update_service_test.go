//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release        *GitHubRelease
	recentReleases []*GitHubRelease
	recentErr      error
	latestErr      error
	latestRepo     string
	recentRepo     string
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	s.latestRepo = repo
	return s.release, s.latestErr
}

func (s *updateServiceGitHubClientStub) FetchRecentReleases(_ context.Context, repo string, _ int) ([]*GitHubRelease, error) {
	s.recentRepo = repo
	return s.recentReleases, s.recentErr
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func newRollbackTestService(current string, releases []*GitHubRelease) *UpdateService {
	return NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentReleases: releases},
		current,
		"release",
	)
}

func TestUpdateServiceListRollbackVersionsFiltersAndCaps(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148", PublishedAt: "2026-07-09T00:00:00Z"},                       // newer than current: excluded
		{TagName: "v0.1.147", PublishedAt: "2026-07-08T00:00:00Z"},                       // current: excluded
		{TagName: "v0.1.146-rc1", PublishedAt: "2026-07-07T12:00:00Z", Prerelease: true}, // prerelease: excluded
		{TagName: "v0.1.146", PublishedAt: "2026-07-07T00:00:00Z"},
		{TagName: "v0.1.145", PublishedAt: "2026-07-06T00:00:00Z", Draft: true}, // draft: excluded
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"},
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"}, // duplicate: excluded
		{TagName: "v0.1.143", PublishedAt: "2026-07-04T00:00:00Z"},
		{TagName: "v0.1.142", PublishedAt: "2026-07-03T00:00:00Z"}, // beyond cap of 3: excluded
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.144", versions[1].Version)
	require.Equal(t, "0.1.143", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsSortsUnorderedInput(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.144"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.145", versions[1].Version)
	require.Equal(t, "0.1.144", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsEmptyWhenNoneOlder(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.148"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Empty(t, versions)
}

func TestUpdateServiceListRollbackVersionsPropagatesFetchError(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentErr: errors.New("github unavailable")},
		"0.1.147",
		"release",
	)

	_, err := svc.ListRollbackVersions(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "github unavailable")
}

func TestUpdateServiceRollbackToVersionRejectsDisallowedTargets(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148"},
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
		{TagName: "v0.1.144"},
		{TagName: "v0.1.143"},
		{TagName: "v0.1.142"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	for _, target := range []string{
		"",         // empty
		"0.1.147",  // current version
		"v0.1.147", // current version with prefix
		"0.1.148",  // newer than current
		"0.1.142",  // older than the 3 most recent
		"9.9.9",    // nonexistent
	} {
		err := svc.RollbackToVersion(context.Background(), target)
		require.ErrorIs(t, err, ErrRollbackVersionNotAllowed, "target %q should be rejected", target)
	}
}

func TestUpdateServiceRollbackToVersionAcceptsVPrefix(t *testing.T) {
	// No platform asset in the release: the target passes the allowlist check
	// and fails later at asset lookup, proving the version itself was accepted.
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	err := svc.RollbackToVersion(context.Background(), "v0.1.146")

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrRollbackVersionNotAllowed)
	require.Contains(t, err.Error(), "no compatible release found")
}

func TestCompareVersionsForkRevisions(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            int
	}{
		{"0.2.9-zy.1", "0.2.9-zy.2", -1},
		{"v0.2.9-zy.2", "v0.2.9-zy.10", -1},
		{"0.2.9-zy.99", "0.2.10-zy.1", -1},
		{"0.2.10-zy.1", "0.2.9-zy.99", 1},
		{"0.2.9-zy.1", "v0.2.9-zy.1", 0},
		{"0.2.9", "0.2.9-zy.1", -1},
		{"0.2.9-zy.1", "0.2.9", 1},
		{"0.2.9+build", "0.2.9-zy.1", -1},
		{"0.2.9-zy.1+build", "0.2.9-zy.1", 0},
		{"0.2.9", "0.2.10", -1},
		{"0.2.9-rc.1", "0.2.9", -1},
	} {
		t.Run(tc.current+"_to_"+tc.latest, func(t *testing.T) {
			require.Equal(t, tc.want, compareVersions(tc.current, tc.latest))
		})
	}
}

func TestUpdateServiceForkDetectionAndCache(t *testing.T) {
	client := &updateServiceGitHubClientStub{release: &GitHubRelease{
		TagName: "v0.2.9-zy.10",
		HTMLURL: "https://github.com/zhiylee/sub2api/releases/tag/v0.2.9-zy.10",
	}}
	cache := &updateServiceCacheStub{}
	svc := NewUpdateService(cache, client, "0.2.9-zy.2", "release")
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	require.Equal(t, "0.2.9-zy.10", info.LatestVersion)
	require.Equal(t, "zhiylee/sub2api", client.latestRepo)

	client.latestErr = errors.New("offline")
	cached, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, cached.Cached)
	require.True(t, cached.HasUpdate)
	require.Equal(t, info.ReleaseInfo.HTMLURL, cached.ReleaseInfo.HTMLURL)

	// A restart on the installed version must no longer report an update even
	// while sharing the previously populated Redis cache.
	restarted := NewUpdateService(cache, client, "0.2.9-zy.10", "release")
	current, err := restarted.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.False(t, current.HasUpdate)
}

func TestUpdateServiceRejectsCommunityCacheEvenOnFetchFailure(t *testing.T) {
	for _, repo := range []string{"", "Wei-Shaw/sub2api"} {
		t.Run(repo, func(t *testing.T) {
			data, err := json.Marshal(map[string]any{
				"repository": repo, "latest": "99.0.0", "timestamp": time.Now().Unix(),
			})
			require.NoError(t, err)
			cache := &updateServiceCacheStub{data: string(data)}
			client := &updateServiceGitHubClientStub{latestErr: errors.New("offline")}
			svc := NewUpdateService(cache, client, "0.2.9-zy.1", "release")
			info, err := svc.CheckUpdate(context.Background(), true)
			require.NoError(t, err)
			require.False(t, info.HasUpdate)
			require.False(t, info.Cached)
			require.Nil(t, info.ReleaseInfo)
			require.Contains(t, info.Warning, "offline")
		})
	}
}

func TestUpdateServiceRejectsInvalidLatestRelease(t *testing.T) {
	for _, release := range []*GitHubRelease{
		nil, {TagName: "main"}, {TagName: "v0.2.9-zy.2", Draft: true},
		{TagName: "v0.2.9-zy.2-rc.1", Prerelease: true},
	} {
		svc := NewUpdateService(&updateServiceCacheStub{}, &updateServiceGitHubClientStub{release: release}, "0.2.9-zy.1", "release")
		info, err := svc.CheckUpdate(context.Background(), true)
		require.NoError(t, err)
		require.False(t, info.HasUpdate)
		require.NotEmpty(t, info.Warning)
	}
}

func TestUpdateServiceForkRollbackOrdering(t *testing.T) {
	client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{
		{TagName: "v0.2.9-zy.2"},
		{TagName: "v0.2.9-zy.11"},
		{TagName: "v0.2.9-zy.9", Prerelease: true},
		{TagName: "v0.2.9-zy.10"},
		{TagName: "v0.2.10-zy.1"},
		{TagName: "v0.2.9-zy.1"},
		{TagName: "v0.2.9-zy.3", Draft: true},
		{TagName: "invalid"},
	}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.9-zy.11", "release")
	versions, err := svc.ListRollbackVersions(context.Background())
	require.NoError(t, err)
	require.Equal(t, "zhiylee/sub2api", client.recentRepo)
	require.Equal(t, []RollbackVersion{
		{Version: "0.2.9-zy.10"}, {Version: "0.2.9-zy.2"}, {Version: "0.2.9-zy.1"},
	}, versions)
	// The chosen revision must reach asset lookup, not be rejected as equal.
	err = svc.RollbackToVersion(context.Background(), "v0.2.9-zy.10")
	require.ErrorContains(t, err, "no compatible release found")
	require.NotErrorIs(t, err, ErrRollbackVersionNotAllowed)
}
