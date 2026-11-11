package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/distribution"

func hasDownstreamUpdate(current, latest string) bool {
	order, ok := distribution.CompareVersions(current, latest)
	return ok && order < 0
}
