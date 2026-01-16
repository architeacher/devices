package decorator

import (
	"context"
	"time"
)

type (
	// CacheStatus represents the status of a cache operation.
	CacheStatus string

	// cacheStatusKey is the context key for cache status.
	cacheStatusKey struct{}

	// CacheConfig holds configuration for the caching decorator.
	CacheConfig struct {
		Enabled bool
		TTL     time.Duration
	}

	// CacheGetter retrieves items from cache.
	CacheGetter[Q Query, R Result] interface {
		Get(ctx context.Context, query Q) (R, bool, error)
	}

	// CacheSetter stores items in cache.
	CacheSetter[Q Query, R Result] interface {
		Set(ctx context.Context, query Q, result R, ttl time.Duration) error
	}

	// Cache combines getter and setter operations.
	Cache[Q Query, R Result] interface {
		CacheGetter[Q, R]
		CacheSetter[Q, R]
	}

	queryCachingDecorator[Q Query, R Result] struct {
		base   QueryHandler[Q, R]
		cache  Cache[Q, R]
		config CacheConfig
	}
)

const (
	CacheStatusHit    CacheStatus = "HIT"
	CacheStatusMiss   CacheStatus = "MISS"
	CacheStatusBypass CacheStatus = "BYPASS"
	CacheStatusError  CacheStatus = "ERROR"
)

// WithCacheStatus adds cache status to context.
func WithCacheStatus(ctx context.Context, status CacheStatus) context.Context {
	return context.WithValue(ctx, cacheStatusKey{}, status)
}

// GetCacheStatus retrieves cache status from context.
func GetCacheStatus(ctx context.Context) CacheStatus {
	if status, ok := ctx.Value(cacheStatusKey{}).(CacheStatus); ok {
		return status
	}

	return CacheStatusBypass
}

// NewQueryCachingDecorator creates a new caching decorator for queries.
func NewQueryCachingDecorator[Q Query, R Result](
	base QueryHandler[Q, R],
	cache Cache[Q, R],
	config CacheConfig,
) QueryHandler[Q, R] {
	return queryCachingDecorator[Q, R]{
		base:   base,
		cache:  cache,
		config: config,
	}
}

func (d queryCachingDecorator[Q, R]) Execute(ctx context.Context, query Q) (R, error) {
	var zero R

	if !d.config.Enabled || d.cache == nil {
		ctx = WithCacheStatus(ctx, CacheStatusBypass)

		return d.base.Execute(ctx, query)
	}

	cached, hit, err := d.cache.Get(ctx, query)
	if err == nil && hit {
		ctx = WithCacheStatus(ctx, CacheStatusHit)

		return cached, nil
	}

	result, err := d.base.Execute(ctx, query)
	if err != nil {
		ctx = WithCacheStatus(ctx, CacheStatusMiss)

		return zero, err
	}

	go func() {
		bgCtx := context.Background()
		_ = d.cache.Set(bgCtx, query, result, d.config.TTL)
	}()

	ctx = WithCacheStatus(ctx, CacheStatusMiss)

	return result, nil
}
