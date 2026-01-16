//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

//counterfeiter:generate -o ../mocks/devices_cache.go . DevicesCache

import (
	"context"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
)

type (
	// CacheStatus represents the cache hit/miss status.
	CacheStatus string

	// CacheResult holds the result of a cache operation along with metadata.
	CacheResult[T any] struct {
		Data     T
		Hit      bool
		Key      string
		TTL      time.Duration
		CachedAt time.Time
	}
)

const (
	CacheStatusHit    CacheStatus = "HIT"
	CacheStatusMiss   CacheStatus = "MISS"
	CacheStatusBypass CacheStatus = "BYPASS"
	CacheStatusStale  CacheStatus = "STALE"
)

// DevicesCache defines the interface for device caching operations.
type DevicesCache interface {
	// GetDevice retrieves a device from the cache by ID.
	// Returns a CacheResult with Hit=false if the device is not cached.
	GetDevice(ctx context.Context, id model.DeviceID) (*CacheResult[*model.Device], error)

	// SetDevice stores a device in the cache with the given TTL.
	SetDevice(ctx context.Context, device *model.Device, ttl time.Duration) error

	// InvalidateDevice removes a device from the cache.
	InvalidateDevice(ctx context.Context, id model.DeviceID) error

	// GetDeviceList retrieves a device list from the cache based on filter.
	// Returns a CacheResult with Hit=false if the list is not cached.
	GetDeviceList(ctx context.Context, filter model.DeviceFilter) (*CacheResult[*model.DeviceList], error)

	// SetDeviceList stores a device list in the cache with the given TTL.
	SetDeviceList(ctx context.Context, list *model.DeviceList, filter model.DeviceFilter, ttl time.Duration) error

	// InvalidateAllLists removes all device list caches.
	InvalidateAllLists(ctx context.Context) error

	// PurgeAll removes all device-related caches.
	PurgeAll(ctx context.Context) error

	// PurgeByPattern removes caches matching the given pattern.
	// Returns the number of keys deleted.
	PurgeByPattern(ctx context.Context, pattern string) (int64, error)

	// IsHealthy checks if the cache is available.
	IsHealthy(ctx context.Context) bool
}
