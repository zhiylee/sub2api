package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/distribution"
)

// UpstreamReleaseInfo is display-only. Installation assets never cross this boundary.
type UpstreamReleaseInfo struct {
	BaseVersion   string `json:"base_version"`
	LatestVersion string `json:"latest_version"`
	HasNewRelease bool   `json:"has_new_release"`
	Name          string `json:"name"`
	PublishedAt   string `json:"published_at"`
	HTMLURL       string `json:"html_url"`
	Cached        bool   `json:"cached"`
	Warning       string `json:"warning,omitempty"`
}

type upstreamReleaseCacheEntry struct {
	Repository string              `json:"repository"`
	Info       UpstreamReleaseInfo `json:"info"`
	Timestamp  int64               `json:"timestamp"`
}

// CheckUpstreamRelease runs independently of CheckUpdate and PerformUpdate.
func (s *UpdateService) CheckUpstreamRelease(ctx context.Context, force bool) *UpstreamReleaseInfo {
	if !force {
		if cached, err := s.getUpstreamReleaseFromCache(ctx); err == nil {
			return cached
		}
	}

	release, err := s.githubClient.FetchLatestRelease(ctx, distribution.UpstreamRepository)
	if err == nil && (release == nil || release.Draft || release.Prerelease || !distribution.IsUpstreamReleaseTag(release.TagName)) {
		err = fmt.Errorf("no stable upstream release found in %s", distribution.UpstreamRepository)
	}
	if err != nil {
		if cached, cacheErr := s.getUpstreamReleaseFromCache(ctx); cacheErr == nil {
			cached.Warning = "Using cached data: " + err.Error()
			return cached
		}
		info := &UpstreamReleaseInfo{HTMLURL: distribution.UpstreamReleasesURL, Warning: err.Error()}
		s.setUpstreamBaseline(info)
		return info
	}

	info := &UpstreamReleaseInfo{
		LatestVersion: strings.TrimPrefix(release.TagName, "v"),
		Name:          release.Name,
		PublishedAt:   release.PublishedAt,
		HTMLURL:       release.HTMLURL,
	}
	if info.HTMLURL == "" {
		info.HTMLURL = distribution.UpstreamReleasesURL
	}
	s.setUpstreamBaseline(info)
	data, _ := json.Marshal(upstreamReleaseCacheEntry{
		Repository: distribution.UpstreamRepository,
		Info:       *info,
		Timestamp:  time.Now().Unix(),
	})
	_ = s.cache.SetUpstreamReleaseInfo(ctx, string(data), time.Duration(updateCacheTTL)*time.Second)
	return info
}

func (s *UpdateService) getUpstreamReleaseFromCache(ctx context.Context) (*UpstreamReleaseInfo, error) {
	data, err := s.cache.GetUpstreamReleaseInfo(ctx)
	if err != nil {
		return nil, err
	}
	var entry upstreamReleaseCacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}
	if entry.Repository != distribution.UpstreamRepository || !distribution.IsUpstreamReleaseTag("v"+entry.Info.LatestVersion) {
		return nil, fmt.Errorf("invalid upstream release cache")
	}
	if time.Now().Unix()-entry.Timestamp > updateCacheTTL {
		return nil, fmt.Errorf("upstream release cache expired")
	}
	entry.Info.Cached = true
	s.setUpstreamBaseline(&entry.Info)
	return &entry.Info, nil
}

func (s *UpdateService) setUpstreamBaseline(info *UpstreamReleaseInfo) {
	info.BaseVersion, _, _ = distribution.ParseVersion(s.currentVersion)
	order, ok := distribution.CompareVersions(info.BaseVersion, info.LatestVersion)
	info.HasNewRelease = ok && order < 0
}
