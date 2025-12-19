package ports

import (
	"context"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
)

// DevicesService defines the interface for device operations.
type DevicesService interface {
	// CreateDevice creates a new device.
	CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error)

	// GetDevice retrieves a device by ID.
	GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error)

	// ListDevices retrieves a paginated list of devices with optional filters.
	ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)

	// UpdateDevice fully updates a device.
	UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error)

	// PatchDevice partially updates a device.
	PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error)

	// DeleteDevice deletes a device by ID.
	DeleteDevice(ctx context.Context, id model.DeviceID) error
}
