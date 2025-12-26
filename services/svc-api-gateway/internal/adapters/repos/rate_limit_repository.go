package repos

import (
	"context"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/throttled/throttled/v2"
)

const (
	rateLimitKeyPrefix = "ratelimit:"
)

// RateLimitStore implements throttled.GCRAStoreCtx using KeydbClient.
type RateLimitStore struct {
	client *infrastructure.KeydbClient
	prefix string
}

// NewRateLimitStore creates a new rate limit store.
func NewRateLimitStore(client *infrastructure.KeydbClient) (throttled.GCRAStoreCtx, error) {
	return &RateLimitStore{
		client: client,
		prefix: rateLimitKeyPrefix,
	}, nil
}

// GetWithTime retrieves a value and its timestamp.
func (s *RateLimitStore) GetWithTime(ctx context.Context, key string) (int64, time.Time, error) {
	return s.client.GetInt64(ctx, s.prefix+key)
}

// SetIfNotExistsWithTTL sets a value if the key doesn't exist.
func (s *RateLimitStore) SetIfNotExistsWithTTL(ctx context.Context, key string, value int64, ttl time.Duration) (bool, error) {
	return s.client.SetInt64NX(ctx, s.prefix+key, value, ttl)
}

// CompareAndSwapWithTTL atomically updates a value if it matches the expected old value.
func (s *RateLimitStore) CompareAndSwapWithTTL(ctx context.Context, key string, old, new int64, ttl time.Duration) (bool, error) {
	return s.client.CompareAndSwapInt64(ctx, s.prefix+key, old, new, ttl)
}
