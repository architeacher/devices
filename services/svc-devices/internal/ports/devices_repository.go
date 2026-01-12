package ports

import (
	"context"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
)

type (
	Saver interface {
		// Create stores a new device in the database.
		Create(ctx context.Context, device *model.Device) error
	}

	Fetcher interface {
		// FetchByID retrieves a device by its ID.
		FetchByID(ctx context.Context, id model.DeviceID) (*model.Device, error)
	}

	Finder interface {
		// List retrieves a paginated list of devices with optional filters.
		List(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
	}

	Updater interface {
		// Update updates an existing device in the database.
		Update(ctx context.Context, device *model.Device) error
	}

	Deleter interface {
		// Delete removes a device from the database by its ID.
		Delete(ctx context.Context, id model.DeviceID) error
	}

	// DeviceRepository defines the interface for device persistence operations.
	DeviceRepository interface {
		Saver
		Fetcher
		Finder
		Updater
		Deleter
	}
)
