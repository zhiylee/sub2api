package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/distribution"
)

const upstreamReleaseCacheKey = "update:" + distribution.UpstreamRepository + ":latest"

func (c *updateCache) GetUpstreamReleaseInfo(ctx context.Context) (string, error) {
	return c.rdb.Get(ctx, upstreamReleaseCacheKey).Result()
}

func (c *updateCache) SetUpstreamReleaseInfo(ctx context.Context, data string, ttl time.Duration) error {
	return c.rdb.Set(ctx, upstreamReleaseCacheKey, data, ttl).Err()
}
