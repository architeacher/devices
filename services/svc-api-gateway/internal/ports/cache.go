//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

//counterfeiter:generate -o ../mocks/idempotency_cache.go . IdempotencyCache

import (
	"context"
	"time"
)

// CachedResponse represents a cached HTTP response.
type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	CreatedAt  time.Time         `json:"created_at"`
}

// IdempotencyCache defines the interface for idempotency caching operations.
type IdempotencyCache interface {
	// Get retrieves a cached response by idempotency key.
	// Returns nil, nil if the key does not exist.
	Get(ctx context.Context, key string) (*CachedResponse, error)

	// Set stores a response with the given idempotency key.
	Set(ctx context.Context, key string, response *CachedResponse, ttl time.Duration) error

	// SetLock acquires a processing lock for the given key.
	// Returns true if the lock was acquired, false if already locked.
	SetLock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// ReleaseLock releases the processing lock.
	ReleaseLock(ctx context.Context, key string) error

	// IsHealthy checks if the cache is available.
	IsHealthy(ctx context.Context) bool
}
