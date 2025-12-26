package repos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/redis/go-redis/v9"
)

const (
	lockSuffix = ":lock"
	lockValue  = "processing"
)

// IdempotencyRepository implements the IdempotencyCache interface using KeyDB/Redis.
type IdempotencyRepository struct {
	client *infrastructure.KeydbClient
}

// NewIdempotencyRepository creates a new idempotency repository.
func NewIdempotencyRepository(client *infrastructure.KeydbClient) (*IdempotencyRepository, error) {
	return &IdempotencyRepository{
		client: client,
	}, nil
}

// Get retrieves a cached response by idempotency key.
func (r *IdempotencyRepository) Get(ctx context.Context, key string) (*ports.CachedResponse, error) {
	data, err := r.client.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, fmt.Errorf("getting cached response: %w", err)
	}

	var response ports.CachedResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("unmarshalling cached response: %w", err)
	}

	return &response, nil
}

// Set stores a response with the given idempotency key.
func (r *IdempotencyRepository) Set(ctx context.Context, key string, response *ports.CachedResponse, ttl time.Duration) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshalling response: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl); err != nil {
		return fmt.Errorf("setting cached response: %w", err)
	}

	return nil
}

// SetLock acquires a processing lock for the given key.
func (r *IdempotencyRepository) SetLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := key + lockSuffix

	acquired, err := r.client.Lock(ctx, lockKey, lockValue, ttl)
	if err != nil {
		return false, fmt.Errorf("acquiring lock: %w", err)
	}

	return acquired, nil
}

// ReleaseLock releases the processing lock.
func (r *IdempotencyRepository) ReleaseLock(ctx context.Context, key string) error {
	lockKey := key + lockSuffix

	if err := r.client.Delete(ctx, lockKey); err != nil {
		return fmt.Errorf("releasing lock: %w", err)
	}

	return nil
}

// IsHealthy checks if the cache is available.
func (r *IdempotencyRepository) IsHealthy(ctx context.Context) bool {
	return r.client.IsHealthy(ctx)
}

// Close closes the underlying Redis client connection.
func (r *IdempotencyRepository) Close() error {
	return r.client.Close()
}
