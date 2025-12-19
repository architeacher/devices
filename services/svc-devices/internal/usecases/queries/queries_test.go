package queries_test

import (
	"context"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/queries"
	"github.com/stretchr/testify/require"
)

type mockDevicesService struct {
	devices        map[string]*model.Device
	getDeviceFn    func(ctx context.Context, id model.DeviceID) (*model.Device, error)
	listDevicesFn  func(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
	createDeviceFn func(ctx context.Context, name, brand string, state model.State) (*model.Device, error)
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

	return nil, model.ErrDeviceNotFound
}

func (m *mockDevicesService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	if m.patchDeviceFn != nil {
		return m.patchDeviceFn(ctx, id, updates)
	}

	return nil, model.ErrDeviceNotFound
}

func (m *mockDevicesService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id)
	}

	return model.ErrDeviceNotFound
}

func TestGetDeviceQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mockDevicesService) model.DeviceID
		expectError bool
	}{
		{
			name: "successfully get device",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				m.devices[device.ID.String()] = device

				return device.ID
			},
			expectError: false,
		},
		{
			name: "device not found",
			setupSvc: func(_ *mockDevicesService) model.DeviceID {
				return model.NewDeviceID()
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			deviceID := tc.setupSvc(svc)

			handler := queries.NewGetDeviceQueryHandler(svc, log, tp, mc)
			ctx := context.Background()

			query := queries.GetDeviceQuery{ID: deviceID}

			device, err := handler.Execute(ctx, query)

			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, device)
			} else {
				require.NoError(t, err)
				require.NotNil(t, device)
				require.Equal(t, deviceID, device.ID)
			}
		})
	}
}

func TestListDevicesQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name          string
		setupSvc      func(*mockDevicesService)
		filter        model.DeviceFilter
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
			filter:        model.DefaultDeviceFilter(),
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
			filter: func() model.DeviceFilter {
				f := model.DefaultDeviceFilter()
				brand := "Brand A"
				f.Brand = &brand

				return f
			}(),
			expectedCount: 1,
		},
		{
			name: "filter by state",
			setupSvc: func(m *mockDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				device2 := model.NewDevice("Device 2", "Brand B", model.StateInUse)
				device3 := model.NewDevice("Device 3", "Brand C", model.StateAvailable)
				m.devices[device1.ID.String()] = device1
				m.devices[device2.ID.String()] = device2
				m.devices[device3.ID.String()] = device3
			},
			filter: func() model.DeviceFilter {
				f := model.DefaultDeviceFilter()
				state := model.StateAvailable
				f.State = &state

				return f
			}(),
			expectedCount: 2,
		},
		{
			name: "empty list",
			setupSvc: func(_ *mockDevicesService) {
			},
			filter:        model.DefaultDeviceFilter(),
			expectedCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			tc.setupSvc(svc)

			handler := queries.NewListDevicesQueryHandler(svc, log, tp, mc)
			ctx := context.Background()

			query := queries.ListDevicesQuery{Filter: tc.filter}

			list, err := handler.Execute(ctx, query)

			require.NoError(t, err)
			require.NotNil(t, list)
			require.Len(t, list.Devices, tc.expectedCount)
		})
	}
}

func TestFetchLivenessQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	handler := queries.NewFetchLivenessQueryHandler(log, tp, mc)
	ctx := context.Background()

	result, err := handler.Execute(ctx, queries.FetchLivenessQuery{})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "ok", result.Status)
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

func TestFetchReadinessQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name          string
		dbHealthy     bool
		expectedReady bool
	}{
		{
			name:          "service is ready when db is healthy",
			dbHealthy:     true,
			expectedReady: true,
		},
		{
			name:          "service is not ready when db is unhealthy",
			dbHealthy:     false,
			expectedReady: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbChecker := &mockDBHealthChecker{healthy: tc.dbHealthy}
			handler := queries.NewFetchReadinessQueryHandler(dbChecker, log, tp, mc)
			ctx := context.Background()

			result, err := handler.Execute(ctx, queries.FetchReadinessQuery{})

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.expectedReady, result.Ready)
		})
	}
}
