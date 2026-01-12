//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

//counterfeiter:generate -o ../mocks/devices_service.go . DevicesService

import (
	"context"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
)

// DevicesService defines the interface for device business operations.
type DevicesService interface {
	// CreateDevice creates a new device with the given parameters.
	CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error)

	// GetDevice retrieves a device by its ID.
	GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error)

	// ListDevices retrieves a paginated list of devices with optional filters.
	ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)

	// UpdateDevice fully updates a device with the given parameters.
	UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error)

	// PatchDevice partially updates a device with the given updates.
	PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error)

	// DeleteDevice deletes a device by its ID.
	DeleteDevice(ctx context.Context, id model.DeviceID) error
}
