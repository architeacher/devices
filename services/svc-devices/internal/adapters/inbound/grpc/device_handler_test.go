package grpc_test

import (
	"context"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockDevicesService struct {
	devices        map[string]*model.Device
	createDeviceFn func(ctx context.Context, name, brand string, state model.State) (*model.Device, error)
	getDeviceFn    func(ctx context.Context, id model.DeviceID) (*model.Device, error)
	listDevicesFn  func(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
	updateDeviceFn func(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error)
	patchDeviceFn  func(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error)
	deleteDeviceFn func(ctx context.Context, id model.DeviceID) error
}

func newMockDevicesService() *mockDevicesService {
	return &mockDevicesService{
		devices: make(map[string]*model.Device),
	}
}

func (m *mockDevicesService) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(ctx, name, brand, state)
	}

	device := model.NewDevice(name, brand, state)
	m.devices[device.ID.String()] = device

	return device, nil
}

func (m *mockDevicesService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}

	device, ok := m.devices[id.String()]
	if !ok {
		return nil, model.ErrDeviceNotFound
	}

	return device, nil
}

func (m *mockDevicesService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	if m.listDevicesFn != nil {
		return m.listDevicesFn(ctx, filter)
	}

	devices := make([]*model.Device, 0)

	for _, d := range m.devices {
		if filter.Brand != nil && d.Brand != *filter.Brand {
			continue
		}

		if filter.State != nil && d.State != *filter.State {
			continue
		}

		devices = append(devices, d)
	}

	return &model.DeviceList{
		Devices: devices,
		Pagination: model.Pagination{
			Page:       filter.Page,
			Size:       filter.Size,
			TotalItems: uint(len(devices)),
			TotalPages: 1,
		},
		Filters: filter,
	}, nil
}

func (m *mockDevicesService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(ctx, id, name, brand, state)
	}

	device, ok := m.devices[id.String()]
	if !ok {
		return nil, model.ErrDeviceNotFound
	}

	if err := device.Update(name, brand, state); err != nil {
		return nil, err
	}

	m.devices[id.String()] = device

	return device, nil
}

func (m *mockDevicesService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	if m.patchDeviceFn != nil {
		return m.patchDeviceFn(ctx, id, updates)
	}

	device, ok := m.devices[id.String()]
	if !ok {
		return nil, model.ErrDeviceNotFound
	}

	if err := device.Patch(updates); err != nil {
		return nil, err
	}

	m.devices[id.String()] = device

	return device, nil
}

func (m *mockDevicesService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id)
	}

	device, ok := m.devices[id.String()]
	if !ok {
		return model.ErrDeviceNotFound
	}

	if !device.CanDelete() {
		return model.ErrCannotDeleteInUseDevice
	}

	delete(m.devices, id.String())

	return nil
}

type mockDBHealthChecker struct {
	healthy bool
}

func (m *mockDBHealthChecker) Ping(_ context.Context) error {
	if !m.healthy {
		return model.ErrDatabaseConnection
	}

	return nil
}

func createTestApp(svc *mockDevicesService, dbChecker *mockDBHealthChecker) *usecases.Application {
	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	return usecases.NewApplication(svc, dbChecker, log, tp, mc)
}

func TestDeviceHandler_CreateDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		request      *devicev1.CreateDeviceRequest
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully create device",
			request: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "missing name returns invalid argument",
			request: &devicev1.CreateDeviceRequest{
				Name:  "",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
		{
			name: "missing brand returns invalid argument",
			request: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			dbChecker := &mockDBHealthChecker{healthy: true}
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.CreateDevice(t.Context(), tc.request)

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Device)
				require.NotEmpty(t, resp.Device.Id)
				require.Equal(t, tc.request.Name, resp.Device.Name)
				require.Equal(t, tc.request.Brand, resp.Device.Brand)
			}
		})
	}
}

func TestDeviceHandler_GetDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mockDevicesService) string
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully get device",
			setupSvc: func(m *mockDevicesService) string {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				m.devices[device.ID.String()] = device

				return device.ID.String()
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "device not found",
			setupSvc: func(_ *mockDevicesService) string {
				return model.NewDeviceID().String()
			},
			expectedCode: codes.NotFound,
			expectError:  true,
		},
		{
			name: "invalid device ID",
			setupSvc: func(_ *mockDevicesService) string {
				return "invalid-uuid"
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
		{
			name: "empty device ID",
			setupSvc: func(_ *mockDevicesService) string {
				return ""
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			dbChecker := &mockDBHealthChecker{healthy: true}
			deviceID := tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.GetDevice(t.Context(), &devicev1.GetDeviceRequest{Id: deviceID})

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Device)
				require.Equal(t, deviceID, resp.Device.Id)
			}
		})
	}
}

func TestDeviceHandler_ListDevices(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		setupSvc      func(*mockDevicesService)
		request       *devicev1.ListDevicesRequest
		expectedCount int
	}{
		{
			name: "list all devices",
			setupSvc: func(m *mockDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				device2 := model.NewDevice("Device 2", "Brand B", model.StateInUse)
				m.devices[device1.ID.String()] = device1
				m.devices[device2.ID.String()] = device2
			},
			request:       &devicev1.ListDevicesRequest{},
			expectedCount: 2,
		},
		{
			name: "filter by brand",
			setupSvc: func(m *mockDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				device2 := model.NewDevice("Device 2", "Brand B", model.StateInUse)
				m.devices[device1.ID.String()] = device1
				m.devices[device2.ID.String()] = device2
			},
			request: &devicev1.ListDevicesRequest{
				Brand: strPtr("Brand A"),
			},
			expectedCount: 1,
		},
		{
			name: "empty list",
			setupSvc: func(_ *mockDevicesService) {
			},
			request:       &devicev1.ListDevicesRequest{},
			expectedCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			dbChecker := &mockDBHealthChecker{healthy: true}
			tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.ListDevices(t.Context(), tc.request)

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Len(t, resp.Devices, tc.expectedCount)
		})
	}
}

func TestDeviceHandler_UpdateDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mockDevicesService) string
		request      func(id string) *devicev1.UpdateDeviceRequest
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully update device",
			setupSvc: func(m *mockDevicesService) string {
				device := model.NewDevice("Original", "Original Brand", model.StateAvailable)
				m.devices[device.ID.String()] = device

				return device.ID.String()
			},
			request: func(id string) *devicev1.UpdateDeviceRequest {
				return &devicev1.UpdateDeviceRequest{
					Id:    id,
					Name:  "Updated Name",
					Brand: "Updated Brand",
					State: devicev1.DeviceState_DEVICE_STATE_IN_USE,
				}
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "cannot update name of in-use device",
			setupSvc: func(m *mockDevicesService) string {
				device := model.NewDevice("Original", "Original Brand", model.StateInUse)
				m.devices[device.ID.String()] = device

				return device.ID.String()
			},
			request: func(id string) *devicev1.UpdateDeviceRequest {
				return &devicev1.UpdateDeviceRequest{
					Id:    id,
					Name:  "New Name",
					Brand: "Original Brand",
					State: devicev1.DeviceState_DEVICE_STATE_IN_USE,
				}
			},
			expectedCode: codes.FailedPrecondition,
			expectError:  true,
		},
		{
			name: "device not found",
			setupSvc: func(_ *mockDevicesService) string {
				return model.NewDeviceID().String()
			},
			request: func(id string) *devicev1.UpdateDeviceRequest {
				return &devicev1.UpdateDeviceRequest{
					Id:    id,
					Name:  "Name",
					Brand: "Brand",
					State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
				}
			},
			expectedCode: codes.NotFound,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			dbChecker := &mockDBHealthChecker{healthy: true}
			deviceID := tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.UpdateDevice(t.Context(), tc.request(deviceID))

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Device)
			}
		})
	}
}

func TestDeviceHandler_DeleteDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mockDevicesService) string
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully delete device",
			setupSvc: func(m *mockDevicesService) string {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				m.devices[device.ID.String()] = device

				return device.ID.String()
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "cannot delete in-use device",
			setupSvc: func(m *mockDevicesService) string {
				device := model.NewDevice("Test", "Brand", model.StateInUse)
				m.devices[device.ID.String()] = device

				return device.ID.String()
			},
			expectedCode: codes.FailedPrecondition,
			expectError:  true,
		},
		{
			name: "device not found",
			setupSvc: func(_ *mockDevicesService) string {
				return model.NewDeviceID().String()
			},
			expectedCode: codes.NotFound,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			dbChecker := &mockDBHealthChecker{healthy: true}
			deviceID := tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.DeleteDevice(t.Context(), &devicev1.DeleteDeviceRequest{Id: deviceID})

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
