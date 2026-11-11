// Package distribution contains the downstream release channel and version rules.
package distribution

import (
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	Repository          = "zhiylee/sub2api"
	UpstreamRepository  = "Wei-Shaw/sub2api"
	UpstreamReleasesURL = "https://github.com/" + UpstreamRepository + "/releases"
)

var versionPattern = regexp.MustCompile(`^v?((?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*))(?:-zy\.([1-9][0-9]*))?$`)

// ParseVersion accepts a stable upstream base or a downstream zy.N revision.
// A bare upstream version is revision zero in the downstream channel.
func ParseVersion(version string) (base, revision string, ok bool) {
	parts := versionPattern.FindStringSubmatch(version)
	if parts == nil {
		return "", "", false
	}
	revision = parts[2]
	if revision == "" {
		revision = "0"
	}
	return parts[1], revision, true
}

func IsReleaseTag(tag string) bool {
	_, revision, ok := ParseVersion(tag)
	return strings.HasPrefix(tag, "v") && ok && revision != "0"
}

func IsUpstreamReleaseTag(tag string) bool {
	_, revision, ok := ParseVersion(tag)
	return strings.HasPrefix(tag, "v") && ok && revision == "0"
}

// CompareVersions compares bases before numeric revisions, without treating
// zy.N as a prerelease of the bare upstream version. Unknown versions are not ordered.
func CompareVersions(current, latest string) (int, bool) {
	currentBase, currentRevision, currentOK := ParseVersion(current)
	latestBase, latestRevision, latestOK := ParseVersion(latest)
	if !currentOK || !latestOK {
		return 0, false
	}
	if order := semver.Compare("v"+currentBase, "v"+latestBase); order != 0 {
		return order, true
	}
	return semver.Compare("v"+currentBase+"-zy."+currentRevision, "v"+latestBase+"-zy."+latestRevision), true
}
