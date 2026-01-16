package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	appLogger "github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/redis/go-redis/v9"
)

type KeydbClient struct {
	client *redis.Client
	logger appLogger.Logger
	config config.Cache
}

func NewKeyDBClient(config config.Cache, logger appLogger.Logger) *KeydbClient {
	opts := &redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           int(config.DB),
		PoolSize:     int(config.PoolSize),
		MinIdleConns: int(config.MinIdleConns),
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
		MaxRetries:   int(config.MaxRetries),
	}

	client := redis.NewClient(opts)

	return &KeydbClient{
		client: client,
		logger: logger,
		config: config,
	}
}

func (c *KeydbClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *KeydbClient) Close() error {
	return c.client.Close()
}

func (c *KeydbClient) Get(ctx context.Context, key string) ([]byte, error) {
	startTime := time.Now()

	result, err := c.client.Get(ctx, key).Bytes()
	duration := time.Since(startTime)

	c.logger.Debug().
		Str("key", key).
		Int64("duration_ms", duration.Milliseconds()).
		Bool("hit", err == nil).
		Msg("keydb get operation")

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, redis.Nil
		}
		c.logger.Error().
			Err(err).
			Str("key", key).
			Msg("keydb get operation failed")

		return nil, err
	}

	return result, nil
}

func (c *KeydbClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.config.DefaultExpiry
	}

	startTime := time.Now()
	var err error

	defer func() {
		duration := time.Since(startTime)

		c.logger.Debug().
			Str("key", key).
			Str("expiry", ttl.String()).
			Int64("duration_ms", duration.Milliseconds()).
			Bool("success", err == nil).
			Msg("keydb set operation")
	}()

	err = c.client.Set(ctx, key, value, ttl).Err()

	return err
}

func (c *KeydbClient) Lock(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	startTime := time.Now()
	var err error

	defer func() {
		duration := time.Since(startTime)

		c.logger.Debug().
			Str("key", key).
			Str("expiry", ttl.String()).
			Int64("duration_ms", duration.Milliseconds()).
			Bool("success", err == nil).
			Msg("keydb setnx operation")
	}()

	acquired, err := c.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquiring lock: %w", err)
	}

	return acquired, err
}

func (c *KeydbClient) Delete(ctx context.Context, key string) error {
	startTime := time.Now()
	var err error

	defer func() {
		duration := time.Since(startTime)

		c.logger.Debug().
			Str("key", key).
			Int64("duration_ms", duration.Milliseconds()).
			Bool("success", err == nil).
			Msg("keydb delete operation")
	}()

	err = c.client.Del(ctx, key).Err()

	return err
}

func (c *KeydbClient) GetStats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// Get keydb info
	info, err := c.client.Info(ctx, "memory", "stats", "clients").Result()
	if err != nil {
		return nil, err
	}

	stats["redis_info"] = info

	poolStats := c.client.PoolStats()
	stats["pool_stats"] = map[string]any{
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
	}

	return stats, nil
}

// IsHealthy checks if the cache is available.
func (c *KeydbClient) IsHealthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err := c.Ping(ctx)

	return err == nil
}

// GetInt64 retrieves an int64 value and its timestamp from the cache.
func (c *KeydbClient) GetInt64(ctx context.Context, key string) (int64, time.Time, error) {
	val, err := c.client.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, time.Time{}, nil
		}

		return 0, time.Time{}, err
	}

	return val, time.Now(), nil
}

// SetInt64NX sets an int64 value if the key doesn't exist.
func (c *KeydbClient) SetInt64NX(ctx context.Context, key string, value int64, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, ttl).Result()
}

// CompareAndSwapInt64 atomically updates a value if it matches the expected old value.
func (c *KeydbClient) CompareAndSwapInt64(ctx context.Context, key string, old, new int64, ttl time.Duration) (bool, error) {
	script := redis.NewScript(`
		local current = redis.call("GET", KEYS[1])
		if current == false or tonumber(current) ~= tonumber(ARGV[1]) then
			return 0
		end
		redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
		return 1
	`)

	result, err := script.Run(ctx, c.client, []string{key}, old, new, ttl.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

// TTL returns the remaining time-to-live of a key.
func (c *KeydbClient) TTL(ctx context.Context, key string) time.Duration {
	result, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		c.logger.Warn().Err(err).Str("key", key).Msg("failed to get TTL")

		return 0
	}

	return result
}

// Scan iterates over keys matching a pattern.
func (c *KeydbClient) Scan(ctx context.Context, cursor uint64, pattern string, count int64) ([]string, uint64, error) {
	keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, count).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("scanning keys: %w", err)
	}

	return keys, nextCursor, nil
}
