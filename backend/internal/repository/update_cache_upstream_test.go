//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestUpdateCacheReleaseChannelsAreIsolated(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewUpdateCache(client)
	ctx := context.Background()
	require.NoError(t, server.Set("update:latest", "legacy official update"))
	_, err := cache.GetUpdateInfo(ctx)
	require.ErrorIs(t, err, redis.Nil)
	_, err = cache.GetUpstreamReleaseInfo(ctx)
	require.ErrorIs(t, err, redis.Nil)
	require.NoError(t, cache.SetUpdateInfo(ctx, "own update", 20*time.Minute))
	require.NoError(t, cache.SetUpstreamReleaseInfo(ctx, "official notice", 5*time.Minute))
	own, err := cache.GetUpdateInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, "own update", own)
	upstream, err := cache.GetUpstreamReleaseInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, "official notice", upstream)
	server.FastForward(6 * time.Minute)
	_, err = cache.GetUpstreamReleaseInfo(ctx)
	require.ErrorIs(t, err, redis.Nil)
	own, err = cache.GetUpdateInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, "own update", own)
}
