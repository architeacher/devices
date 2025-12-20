package queries_test

import (
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-devices/internal/mocks"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/queries"
	"github.com/stretchr/testify/require"
)

func TestGetDeviceQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mocks.FakeDevicesService) model.DeviceID
		expectError bool
	}{
		{
			name: "successfully get device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				fake.GetDeviceReturns(device, nil)

				return device.ID
			},
			expectError: false,
		},
		{
			name: "device not found",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				fake.GetDeviceReturns(nil, model.ErrDeviceNotFound)

				return model.NewDeviceID()
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			deviceID := tc.setupSvc(svc)

			handler := queries.NewGetDeviceQueryHandler(svc, log, tp, mc)

			query := queries.GetDeviceQuery{ID: deviceID}

			device, err := handler.Execute(t.Context(), query)

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
		setupSvc      func(*mocks.FakeDevicesService)
		filter        model.DeviceFilter
		expectedCount int
	}{
		{
			name: "list all devices",
			setupSvc: func(fake *mocks.FakeDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				device2 := model.NewDevice("Device 2", "Brand B", model.StateInUse)
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{device1, device2},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 2,
						TotalPages: 1,
					},
					Filters: model.DefaultDeviceFilter(),
				}, nil)
			},
			filter:        model.DefaultDeviceFilter(),
			expectedCount: 2,
		},
		{
			name: "filter by brand",
			setupSvc: func(fake *mocks.FakeDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				brand := "Brand A"
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{device1},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 1,
						TotalPages: 1,
					},
					Filters: model.DeviceFilter{Brand: &brand, Page: 1, Size: 10},
				}, nil)
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
			setupSvc: func(fake *mocks.FakeDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				device3 := model.NewDevice("Device 3", "Brand C", model.StateAvailable)
				state := model.StateAvailable
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{device1, device3},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 2,
						TotalPages: 1,
					},
					Filters: model.DeviceFilter{State: &state, Page: 1, Size: 10},
				}, nil)
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
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 0,
						TotalPages: 1,
					},
					Filters: model.DefaultDeviceFilter(),
				}, nil)
			},
			filter:        model.DefaultDeviceFilter(),
			expectedCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			tc.setupSvc(svc)

			handler := queries.NewListDevicesQueryHandler(svc, log, tp, mc)

			query := queries.ListDevicesQuery{Filter: tc.filter}

			list, err := handler.Execute(t.Context(), query)

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

	result, err := handler.Execute(t.Context(), queries.FetchLivenessQuery{})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "ok", result.Status)
}

func TestFetchReadinessQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name          string
		setupChecker  func(*mocks.FakeDatabaseHealthChecker)
		expectedReady bool
	}{
		{
			name: "service is ready when db is healthy",
			setupChecker: func(fake *mocks.FakeDatabaseHealthChecker) {
				fake.PingReturns(nil)
			},
			expectedReady: true,
		},
		{
			name: "service is not ready when db is unhealthy",
			setupChecker: func(fake *mocks.FakeDatabaseHealthChecker) {
				fake.PingReturns(model.ErrDatabaseConnection)
			},
			expectedReady: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			tc.setupChecker(dbChecker)

			handler := queries.NewFetchReadinessQueryHandler(dbChecker, log, tp, mc)

			result, err := handler.Execute(t.Context(), queries.FetchReadinessQuery{})

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.expectedReady, result.Ready)
		})
	}
}
