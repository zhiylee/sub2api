package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type userGroupRateResolver struct {
	repo         UserGroupRateRepository
	cache        *gocache.Cache
	cacheTTL     time.Duration
	sf           *singleflight.Group
	logComponent string
}

func newUserGroupRateResolver(repo UserGroupRateRepository, cache *gocache.Cache, cacheTTL time.Duration, sf *singleflight.Group, logComponent string) *userGroupRateResolver {
	if cacheTTL <= 0 {
		cacheTTL = defaultUserGroupRateCacheTTL
	}
	if cache == nil {
		cache = gocache.New(cacheTTL, time.Minute)
	}
	if logComponent == "" {
		logComponent = "service.gateway"
	}
	if sf == nil {
		sf = &singleflight.Group{}
	}

	return &userGroupRateResolver{
		repo:         repo,
		cache:        cache,
		cacheTTL:     cacheTTL,
		sf:           sf,
		logComponent: logComponent,
	}
}

func (r *userGroupRateResolver) Resolve(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64 {
	if override, ok := r.resolveOverride(ctx, userID, groupID); ok {
		return override
	}
	return groupDefaultMultiplier
}

type noUserGroupRate struct{}

// Cache the override, including its absence, so each request can supply its own fallback.
func (r *userGroupRateResolver) resolveOverride(ctx context.Context, userID, groupID int64) (float64, bool) {
	if r == nil || userID <= 0 || groupID <= 0 {
		return 0, false
	}

	key := fmt.Sprintf("%d:%d", userID, groupID)
	if cached, ok := r.cachedOverride(key); ok {
		rate, found := cached.(float64)
		return rate, found
	}
	if r.repo == nil {
		return 0, false
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if multiplier, ok := r.cachedOverride(key); ok {
			return multiplier, nil
		}

		userGroupRateCacheLoadTotal.Add(1)
		userRate, repoErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		var cached any = noUserGroupRate{}
		if userRate != nil {
			cached = *userRate
		}
		if r.cache != nil {
			r.cache.Set(key, cached, r.cacheTTL)
		}
		return cached, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group rate failed: user=%d group=%d err=%v", userID, groupID, err)
		return 0, false
	}

	multiplier, ok := value.(float64)
	return multiplier, ok
}

func (r *userGroupRateResolver) cachedOverride(key string) (any, bool) {
	if r.cache == nil {
		return nil, false
	}
	cached, _ := r.cache.Get(key)
	switch cached.(type) {
	case float64, noUserGroupRate:
		userGroupRateCacheHitTotal.Add(1)
		return cached, true
	}
	return nil, false
}
