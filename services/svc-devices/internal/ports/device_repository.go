package ports

import (
	"context"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
)

// DeviceRepository defines the interface for device persistence operations.
type DeviceRepository interface {
	// Create stores a new device in the database.
	Create(ctx context.Context, device *model.Device) error

	// GetByID retrieves a device by its ID.
	GetByID(ctx context.Context, id model.DeviceID) (*model.Device, error)

	// List retrieves a paginated list of devices with optional filters.
	List(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)

	// Update updates an existing device in the database.
	Update(ctx context.Context, device *model.Device) error

	// Delete removes a device from the database by its ID.
	Delete(ctx context.Context, id model.DeviceID) error
}
