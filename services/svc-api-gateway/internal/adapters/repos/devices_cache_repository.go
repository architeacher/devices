package repos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/redis/go-redis/v9"
)

const (
	deviceCacheVersion = "v1"
	deviceKeyPrefix    = "device:" + deviceCacheVersion + ":"
	deviceListPrefix   = "devices:list:" + deviceCacheVersion + ":"
)

type (
	// cachedDevice represents a device in JSON format for caching.
	cachedDevice struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Brand     string    `json:"brand"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	// cachedDeviceList represents a device list in JSON format for caching.
	cachedDeviceList struct {
		Devices    []cachedDevice  `json:"devices"`
		Pagination model.Pagination `json:"pagination"`
	}

	// DevicesCacheRepository implements the DevicesCache interface using KeyDB/Redis.
	DevicesCacheRepository struct {
		client *infrastructure.KeydbClient
		logger logger.Logger
	}
)

// NewDevicesCacheRepository creates a new devices cache repository.
func NewDevicesCacheRepository(client *infrastructure.KeydbClient, log logger.Logger) *DevicesCacheRepository {
	return &DevicesCacheRepository{
		client: client,
		logger: log,
	}
}

// GetDevice retrieves a device from the cache by ID.
func (r *DevicesCacheRepository) GetDevice(ctx context.Context, id model.DeviceID) (*ports.CacheResult[*model.Device], error) {
	key := r.deviceKey(id)

	data, err := r.client.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return &ports.CacheResult[*model.Device]{
				Hit: false,
				Key: key,
			}, nil
		}

		return nil, fmt.Errorf("getting cached device: %w", err)
	}

	var cached cachedDevice
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("unmarshalling cached device: %w", err)
	}

	device, err := r.toDomainDevice(cached)
	if err != nil {
		return nil, fmt.Errorf("converting cached device: %w", err)
	}

	ttl := r.client.TTL(ctx, key)

	return &ports.CacheResult[*model.Device]{
		Data:     device,
		Hit:      true,
		Key:      key,
		TTL:      ttl,
		CachedAt: device.UpdatedAt,
	}, nil
}

// SetDevice stores a device in the cache with the given TTL.
func (r *DevicesCacheRepository) SetDevice(ctx context.Context, device *model.Device, ttl time.Duration) error {
	key := r.deviceKey(device.ID)

	cached := r.toCachedDevice(device)
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("marshalling device: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl); err != nil {
		return fmt.Errorf("setting cached device: %w", err)
	}

	return nil
}

// InvalidateDevice removes a device from the cache.
func (r *DevicesCacheRepository) InvalidateDevice(ctx context.Context, id model.DeviceID) error {
	key := r.deviceKey(id)

	if err := r.client.Delete(ctx, key); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("invalidating cached device: %w", err)
	}

	return nil
}

// GetDeviceList retrieves a device list from the cache based on filter.
func (r *DevicesCacheRepository) GetDeviceList(ctx context.Context, filter model.DeviceFilter) (*ports.CacheResult[*model.DeviceList], error) {
	key := r.deviceListKey(filter)

	data, err := r.client.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return &ports.CacheResult[*model.DeviceList]{
				Hit: false,
				Key: key,
			}, nil
		}

		return nil, fmt.Errorf("getting cached device list: %w", err)
	}

	var cached cachedDeviceList
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("unmarshalling cached device list: %w", err)
	}

	list, err := r.toDomainDeviceList(cached, filter)
	if err != nil {
		return nil, fmt.Errorf("converting cached device list: %w", err)
	}

	ttl := r.client.TTL(ctx, key)

	return &ports.CacheResult[*model.DeviceList]{
		Data:     list,
		Hit:      true,
		Key:      key,
		TTL:      ttl,
		CachedAt: time.Now().UTC(),
	}, nil
}

// SetDeviceList stores a device list in the cache with the given TTL.
func (r *DevicesCacheRepository) SetDeviceList(ctx context.Context, list *model.DeviceList, filter model.DeviceFilter, ttl time.Duration) error {
	key := r.deviceListKey(filter)

	cached := r.toCachedDeviceList(list)
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("marshalling device list: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl); err != nil {
		return fmt.Errorf("setting cached device list: %w", err)
	}

	return nil
}

// InvalidateAllLists removes all device list caches.
func (r *DevicesCacheRepository) InvalidateAllLists(ctx context.Context) error {
	_, err := r.purgeByPattern(ctx, fmt.Sprintf("%s*", deviceListPrefix))
	if err != nil {
		return fmt.Errorf("invalidating all device lists: %w", err)
	}

	return nil
}

// PurgeAll removes all device-related caches.
func (r *DevicesCacheRepository) PurgeAll(ctx context.Context) error {
	patterns := []string{
		fmt.Sprintf("%s*", deviceKeyPrefix),
		fmt.Sprintf("%s*", deviceListPrefix),
	}

	for _, pattern := range patterns {
		if _, err := r.purgeByPattern(ctx, pattern); err != nil {
			return fmt.Errorf("purging pattern %s: %w", pattern, err)
		}
	}

	return nil
}

// PurgeByPattern removes caches matching the given pattern.
func (r *DevicesCacheRepository) PurgeByPattern(ctx context.Context, pattern string) (int64, error) {
	return r.purgeByPattern(ctx, pattern)
}

// IsHealthy checks if the cache is available.
func (r *DevicesCacheRepository) IsHealthy(ctx context.Context) bool {
	return r.client.IsHealthy(ctx)
}

func (r *DevicesCacheRepository) deviceKey(id model.DeviceID) string {
	return fmt.Sprintf("%s%s", deviceKeyPrefix, id.String())
}

func (r *DevicesCacheRepository) deviceListKey(filter model.DeviceFilter) string {
	return fmt.Sprintf("%s%s", deviceListPrefix, r.hashFilter(filter))
}

func (r *DevicesCacheRepository) hashFilter(filter model.DeviceFilter) string {
	sortedBrands := make([]string, len(filter.Brands))
	copy(sortedBrands, filter.Brands)
	sort.Strings(sortedBrands)

	sortedStates := make([]string, len(filter.States))
	for index, state := range filter.States {
		sortedStates[index] = state.String()
	}
	sort.Strings(sortedStates)

	sortedSort := make([]string, len(filter.Sort))
	copy(sortedSort, filter.Sort)
	sort.Strings(sortedSort)

	filterKey := fmt.Sprintf(
		"keyword=%s&brands=%s&states=%s&sort=%s&page=%d&size=%d&cursor=%s",
		filter.Keyword,
		strings.Join(sortedBrands, ","),
		strings.Join(sortedStates, ","),
		strings.Join(sortedSort, ","),
		filter.Page,
		filter.Size,
		filter.Cursor,
	)

	hash := sha256.Sum256([]byte(filterKey))

	return hex.EncodeToString(hash[:16])
}

func (r *DevicesCacheRepository) purgeByPattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var totalDeleted int64

	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100)
		if err != nil {
			return totalDeleted, fmt.Errorf("scanning keys: %w", err)
		}

		if len(keys) > 0 {
			for _, key := range keys {
				if err := r.client.Delete(ctx, key); err != nil && !errors.Is(err, redis.Nil) {
					r.logger.Warn().Str("key", key).Err(err).Msg("failed to delete key during purge")
					continue
				}
				totalDeleted++
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return totalDeleted, nil
}

func (r *DevicesCacheRepository) toCachedDevice(device *model.Device) cachedDevice {
	return cachedDevice{
		ID:        device.ID.String(),
		Name:      device.Name,
		Brand:     device.Brand,
		State:     device.State.String(),
		CreatedAt: device.CreatedAt,
		UpdatedAt: device.UpdatedAt,
	}
}

func (r *DevicesCacheRepository) toDomainDevice(cached cachedDevice) (*model.Device, error) {
	id, err := model.ParseDeviceID(cached.ID)
	if err != nil {
		return nil, fmt.Errorf("parsing device ID: %w", err)
	}

	state, err := model.ParseState(cached.State)
	if err != nil {
		return nil, fmt.Errorf("parsing device state: %w", err)
	}

	return &model.Device{
		ID:        id,
		Name:      cached.Name,
		Brand:     cached.Brand,
		State:     state,
		CreatedAt: cached.CreatedAt,
		UpdatedAt: cached.UpdatedAt,
	}, nil
}

func (r *DevicesCacheRepository) toCachedDeviceList(list *model.DeviceList) cachedDeviceList {
	devices := make([]cachedDevice, len(list.Devices))
	for index, device := range list.Devices {
		devices[index] = r.toCachedDevice(device)
	}

	return cachedDeviceList{
		Devices:    devices,
		Pagination: list.Pagination,
	}
}

func (r *DevicesCacheRepository) toDomainDeviceList(cached cachedDeviceList, filter model.DeviceFilter) (*model.DeviceList, error) {
	devices := make([]*model.Device, len(cached.Devices))
	for index := range cached.Devices {
		device, err := r.toDomainDevice(cached.Devices[index])
		if err != nil {
			return nil, fmt.Errorf("converting device at index %d: %w", index, err)
		}
		devices[index] = device
	}

	return &model.DeviceList{
		Devices:    devices,
		Pagination: cached.Pagination,
		Filters:    filter,
	}, nil
}
