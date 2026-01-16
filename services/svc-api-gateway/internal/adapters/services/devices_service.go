package services

import (
	"context"
	"strings"
	"time"

	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	grpcclient "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/outbound/grpc"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const devicesServiceName = "svc-devices"

// DevicesService coordinates device operations using the gRPC outbound adapter.
// It handles domain mapping and error translation.
type DevicesService struct {
	client *grpcclient.Client
}

var (
	_ ports.DevicesService = (*DevicesService)(nil)
	_ ports.HealthChecker  = (*DevicesService)(nil)
)

// NewDevicesService creates a new service that coordinates with the gRPC outbound adapter.
// The client lifecycle is managed by the caller.
func NewDevicesService(client *grpcclient.Client) *DevicesService {
	return &DevicesService{
		client: client,
	}
}

// CreateDevice creates a new device.
func (s *DevicesService) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	req := &devicev1.CreateDeviceRequest{
		Name:  name,
		Brand: brand,
		State: toProtoState(state),
	}

	resp, err := s.client.CreateDevice(ctx, req)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// GetDevice retrieves a device by ID.
func (s *DevicesService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	req := &devicev1.GetDeviceRequest{
		Id: id.String(),
	}

	resp, err := s.client.GetDevice(ctx, req)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// ListDevices retrieves a paginated list of devices with optional filters.
func (s *DevicesService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	req := toProtoListRequest(filter)

	resp, err := s.client.ListDevices(ctx, req)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &model.DeviceList{
		Devices:    toDomainDevices(resp.GetDevices()),
		Pagination: toDomainPagination(resp.GetPagination()),
		Filters:    filter,
	}, nil
}

// UpdateDevice fully updates a device.
func (s *DevicesService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	req := &devicev1.UpdateDeviceRequest{
		Id:    id.String(),
		Name:  name,
		Brand: brand,
		State: toProtoState(state),
	}

	resp, err := s.client.UpdateDevice(ctx, req)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// PatchDevice partially updates a device.
func (s *DevicesService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	req := toProtoPatchRequest(id, updates)

	resp, err := s.client.PatchDevice(ctx, req)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// DeleteDevice deletes a device by ID.
func (s *DevicesService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	req := &devicev1.DeleteDeviceRequest{
		Id: id.String(),
	}

	_, err := s.client.DeleteDevice(ctx, req)
	if err != nil {
		return mapGRPCError(err)
	}

	return nil
}

// Liveness returns the liveness status.
func (s *DevicesService) Liveness(ctx context.Context) (*model.LivenessReport, error) {
	resp, err := s.client.CheckHealth(ctx, &devicev1.HealthCheckRequest{})
	if err != nil {
		return &model.LivenessReport{
			Status:    model.HealthStatusDown,
			Timestamp: time.Now().UTC(),
			Version:   config.ServiceVersion,
		}, nil
	}

	status := model.HealthStatusOK
	if resp.GetStatus() != devicev1.HealthCheckResponse_SERVING_STATUS_SERVING {
		status = model.HealthStatusDown
	}

	return &model.LivenessReport{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Version:   config.ServiceVersion,
	}, nil
}

// Readiness returns the readiness status including dependency checks.
func (s *DevicesService) Readiness(ctx context.Context) (*model.ReadinessReport, error) {
	checks := make(map[string]model.DependencyCheck)
	now := time.Now().UTC()

	resp, err := s.client.CheckHealth(ctx, &devicev1.HealthCheckRequest{})
	if err != nil {
		checks[devicesServiceName] = model.DependencyCheck{
			Status:      model.DependencyStatusDown,
			Message:     err.Error(),
			LastChecked: now,
		}

		return &model.ReadinessReport{
			Status:    model.HealthStatusDown,
			Timestamp: now,
			Version:   config.ServiceVersion,
			Checks:    checks,
		}, nil
	}

	depStatus := model.DependencyStatusUp
	if resp.GetStatus() != devicev1.HealthCheckResponse_SERVING_STATUS_SERVING {
		depStatus = model.DependencyStatusDown
	}

	checks[devicesServiceName] = model.DependencyCheck{
		Status:      depStatus,
		Message:     "ok",
		LastChecked: now,
	}

	overallStatus := model.HealthStatusOK
	if depStatus == model.DependencyStatusDown {
		overallStatus = model.HealthStatusDown
	}

	return &model.ReadinessReport{
		Status:    overallStatus,
		Timestamp: now,
		Version:   config.ServiceVersion,
		Checks:    checks,
	}, nil
}

// Health returns a comprehensive health report.
func (s *DevicesService) Health(ctx context.Context) (*model.HealthReport, error) {
	checks := make(map[string]model.DependencyCheck)
	now := time.Now().UTC()
	cfg := s.client.Config()

	resp, err := s.client.CheckHealth(ctx, &devicev1.HealthCheckRequest{})
	if err != nil {
		checks[devicesServiceName] = model.DependencyCheck{
			Status:      model.DependencyStatusDown,
			Message:     err.Error(),
			LastChecked: now,
		}

		return &model.HealthReport{
			Status:    model.HealthStatusDown,
			Timestamp: now,
			Version: model.VersionInfo{
				API:   cfg.App.APIVersion,
				Build: config.CommitSHA,
			},
			Checks: checks,
		}, nil
	}

	depStatus := model.DependencyStatusUp
	if resp.GetStatus() != devicev1.HealthCheckResponse_SERVING_STATUS_SERVING {
		depStatus = model.DependencyStatusDown
	}

	checks[devicesServiceName] = model.DependencyCheck{
		Status:      depStatus,
		Message:     "ok",
		LastChecked: now,
	}

	overallStatus := model.HealthStatusOK
	if depStatus == model.DependencyStatusDown {
		overallStatus = model.HealthStatusDown
	}

	return &model.HealthReport{
		Status:    overallStatus,
		Timestamp: now,
		Version: model.VersionInfo{
			API:   cfg.App.APIVersion,
			Build: config.CommitSHA,
		},
		Checks: checks,
	}, nil
}

func toProtoState(s model.State) devicev1.DeviceState {
	switch s {
	case model.StateAvailable:
		return devicev1.DeviceState_DEVICE_STATE_AVAILABLE
	case model.StateInUse:
		return devicev1.DeviceState_DEVICE_STATE_IN_USE
	case model.StateInactive:
		return devicev1.DeviceState_DEVICE_STATE_INACTIVE
	default:
		return devicev1.DeviceState_DEVICE_STATE_UNSPECIFIED
	}
}

func toDomainState(s devicev1.DeviceState) model.State {
	switch s {
	case devicev1.DeviceState_DEVICE_STATE_AVAILABLE:
		return model.StateAvailable
	case devicev1.DeviceState_DEVICE_STATE_IN_USE:
		return model.StateInUse
	case devicev1.DeviceState_DEVICE_STATE_INACTIVE:
		return model.StateInactive
	default:
		return model.StateAvailable
	}
}

func toDomainDevice(d *devicev1.Device) *model.Device {
	if d == nil {
		return nil
	}

	id, _ := model.ParseDeviceID(d.GetId())

	device := &model.Device{
		ID:    id,
		Name:  d.GetName(),
		Brand: d.GetBrand(),
		State: toDomainState(d.GetState()),
	}

	if d.GetCreatedAt() != nil {
		device.CreatedAt = d.GetCreatedAt().AsTime()
	}

	if d.GetUpdatedAt() != nil {
		device.UpdatedAt = d.GetUpdatedAt().AsTime()
	}

	return device
}

func toDomainDevices(devices []*devicev1.Device) []*model.Device {
	result := make([]*model.Device, 0, len(devices))
	for _, d := range devices {
		result = append(result, toDomainDevice(d))
	}

	return result
}

func toDomainPagination(p *devicev1.Pagination) model.Pagination {
	if p == nil {
		return model.Pagination{}
	}

	return model.Pagination{
		Page:           uint(p.GetPage()),
		Size:           uint(p.GetSize()),
		TotalItems:     uint(p.GetTotalItems()),
		TotalPages:     uint(p.GetTotalPages()),
		HasNext:        p.GetHasNext(),
		HasPrevious:    p.GetHasPrevious(),
		NextCursor:     p.GetNextCursor(),
		PreviousCursor: p.GetPreviousCursor(),
	}
}

func toProtoListRequest(filter model.DeviceFilter) *devicev1.ListDevicesRequest {
	req := &devicev1.ListDevicesRequest{
		Query:  filter.Keyword,
		Sort:   filter.Sort,
		Page:   uint32(filter.Page),
		Size:   uint32(filter.Size),
		Cursor: filter.Cursor,
	}

	if len(filter.Brands) > 0 {
		req.Brands = filter.Brands
	}

	if len(filter.States) > 0 {
		for _, s := range filter.States {
			req.States = append(req.States, toProtoState(s))
		}
	}

	return req
}

func toProtoPatchRequest(id model.DeviceID, updates map[string]any) *devicev1.PatchDeviceRequest {
	req := &devicev1.PatchDeviceRequest{
		Id: id.String(),
	}

	if name, ok := updates["name"].(string); ok {
		req.Name = &name
	}

	if brand, ok := updates["brand"].(string); ok {
		req.Brand = &brand
	}

	if stateStr, ok := updates["state"].(string); ok {
		if state, err := model.ParseState(stateStr); err == nil {
			protoState := toProtoState(state)
			req.State = &protoState
		}
	}

	if state, ok := updates["state"].(model.State); ok {
		protoState := toProtoState(state)
		req.State = &protoState
	}

	return req
}

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound:
		return model.ErrDeviceNotFound

	case codes.FailedPrecondition:
		msg := st.Message()
		if strings.Contains(msg, "cannot update") {
			return model.ErrCannotUpdateInUseDevice
		}
		if strings.Contains(msg, "cannot delete") {
			return model.ErrCannotDeleteInUseDevice
		}

		return err

	case codes.InvalidArgument:
		return &model.ValidationErrors{
			Errors: []model.ValidationError{
				{
					Message: st.Message(),
					Code:    "invalid_argument",
				},
			},
		}

	case codes.Unavailable:
		return model.ErrServiceUnavailable

	case codes.DeadlineExceeded:
		return model.ErrTimeout

	default:
		return err
	}
}
