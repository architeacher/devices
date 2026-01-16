package repos

import (
	"context"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
)

type (
	// GetDeviceCacheAdapter adapts DevicesCache for GetDeviceQuery.
	GetDeviceCacheAdapter struct {
		cache ports.DevicesCache
	}

	// ListDevicesCacheAdapter adapts DevicesCache for ListDevicesQuery.
	ListDevicesCacheAdapter struct {
		cache ports.DevicesCache
	}
)

// NewGetDeviceCacheAdapter creates a new cache adapter for GetDeviceQuery.
func NewGetDeviceCacheAdapter(cache ports.DevicesCache) *GetDeviceCacheAdapter {
	return &GetDeviceCacheAdapter{cache: cache}
}

// Get retrieves a device from the cache.
func (a *GetDeviceCacheAdapter) Get(ctx context.Context, query queries.GetDeviceQuery) (*model.Device, bool, error) {
	result, err := a.cache.GetDevice(ctx, query.ID)
	if err != nil {
		return nil, false, err
	}

	return result.Data, result.Hit, nil
}

// Set stores a device in the cache.
func (a *GetDeviceCacheAdapter) Set(ctx context.Context, query queries.GetDeviceQuery, result *model.Device, ttl time.Duration) error {
	return a.cache.SetDevice(ctx, result, ttl)
}

// NewListDevicesCacheAdapter creates a new cache adapter for ListDevicesQuery.
func NewListDevicesCacheAdapter(cache ports.DevicesCache) *ListDevicesCacheAdapter {
	return &ListDevicesCacheAdapter{cache: cache}
}

// Get retrieves a device list from the cache.
func (a *ListDevicesCacheAdapter) Get(ctx context.Context, query queries.ListDevicesQuery) (*model.DeviceList, bool, error) {
	result, err := a.cache.GetDeviceList(ctx, query.Filter)
	if err != nil {
		return nil, false, err
	}

	return result.Data, result.Hit, nil
}

// Set stores a device list in the cache.
func (a *ListDevicesCacheAdapter) Set(ctx context.Context, query queries.ListDevicesQuery, result *model.DeviceList, ttl time.Duration) error {
	return a.cache.SetDeviceList(ctx, result, query.Filter, ttl)
}
