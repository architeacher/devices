package devices

import (
	"context"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
)

// TODO: Replace with gRPC client when svc-devices is ready.
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) CreateDevice(_ context.Context, name, brand string, state model.State) (*model.Device, error) {
	return &model.Device{
		ID:        model.NewDeviceID(),
		Name:      name,
		Brand:     brand,
		State:     state,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (c *Client) GetDevice(_ context.Context, id model.DeviceID) (*model.Device, error) {
	return &model.Device{
		ID:        id,
		Name:      "",
		Brand:     "",
		State:     model.StateAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (c *Client) ListDevices(_ context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	return &model.DeviceList{
		Devices: []*model.Device{},
		Pagination: model.Pagination{
			Page:        filter.Page,
			Size:        filter.Size,
			TotalItems:  0,
			TotalPages:  1,
			HasNext:     false,
			HasPrevious: false,
		},
		Filters: filter,
	}, nil
}

func (c *Client) UpdateDevice(_ context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	return &model.Device{
		ID:        id,
		Name:      name,
		Brand:     brand,
		State:     state,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (c *Client) PatchDevice(_ context.Context, id model.DeviceID, _ map[string]any) (*model.Device, error) {
	return &model.Device{
		ID:        id,
		Name:      "",
		Brand:     "",
		State:     model.StateAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (c *Client) DeleteDevice(_ context.Context, _ model.DeviceID) error {
	return nil
}

func (c *Client) Liveness(_ context.Context) (*model.LivenessReport, error) {
	return &model.LivenessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}, nil
}

func (c *Client) Readiness(_ context.Context) (*model.ReadinessReport, error) {
	return &model.ReadinessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Checks: map[string]model.DependencyCheck{
			"storage": {
				Status:      model.DependencyStatusUp,
				LatencyMs:   0,
				Message:     "ok",
				LastChecked: time.Now().UTC(),
			},
		},
	}, nil
}

func (c *Client) Health(_ context.Context) (*model.HealthReport, error) {
	return &model.HealthReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version: model.VersionInfo{
			API:   "1.0.0",
			Build: "development",
			Go:    "1.23",
		},
		Checks: map[string]model.DependencyCheck{
			"storage": {
				Status:      model.DependencyStatusUp,
				LatencyMs:   0,
				Message:     "ok",
				LastChecked: time.Now().UTC(),
			},
		},
		System: model.SystemInfo{
			Goroutines: 1,
			CPUCores:   1,
		},
	}, nil
}
