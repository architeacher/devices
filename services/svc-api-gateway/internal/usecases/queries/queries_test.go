package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
	"github.com/stretchr/testify/require"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

func TestGetDeviceQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	mc := noop.NewMetricsClient()
	tp := otelNoop.NewTracerProvider()

	cases := []struct {
		name        string
		setupSvc    func() (*mocks.FakeDevicesService, model.DeviceID)
		expectError bool
		expectedErr error
	}{
		{
			name: "successfully get device",
			setupSvc: func() (*mocks.FakeDevicesService, model.DeviceID) {
				fake := &mocks.FakeDevicesService{}
				id := model.NewDeviceID()
				expectedDevice := &model.Device{
					ID:    id,
					Name:  "Test Device",
					Brand: "Test Brand",
					State: model.StateAvailable,
				}
				fake.GetDeviceReturns(expectedDevice, nil)

				return fake, id
			},
			expectError: false,
		},
		{
			name: "device not found",
			setupSvc: func() (*mocks.FakeDevicesService, model.DeviceID) {
				fake := &mocks.FakeDevicesService{}
				fake.GetDeviceReturns(nil, model.ErrDeviceNotFound)

				return fake, model.NewDeviceID()
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc, deviceID := tc.setupSvc()

			handler := queries.NewGetDeviceQueryHandler(svc, log, mc, tp)
			query := queries.GetDeviceQuery{ID: deviceID}

			result, err := handler.Execute(t.Context(), query)

			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, result)
				if tc.expectedErr != nil {
					require.ErrorIs(t, err, tc.expectedErr)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, deviceID, result.ID)
			}
		})
	}
}

func TestListDevicesQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	mc := noop.NewMetricsClient()
	tp := otelNoop.NewTracerProvider()

	brand := "Apple"
	state := model.StateAvailable

	cases := []struct {
		name     string
		filter   model.DeviceFilter
		setupSvc func() *mocks.FakeDevicesService
		validate func(*testing.T, *model.DeviceList)
	}{
		{
			name:   "list with default filter",
			filter: model.DefaultDeviceFilter(),
			setupSvc: func() *mocks.FakeDevicesService {
				fake := &mocks.FakeDevicesService{}
				fake.ListDevicesStub = func(_ context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
					return &model.DeviceList{
						Devices: []*model.Device{},
						Pagination: model.Pagination{
							Page:       filter.Page,
							Size:       filter.Size,
							TotalItems: 0,
							TotalPages: 1,
						},
						Filters: filter,
					}, nil
				}

				return fake
			},
			validate: func(t *testing.T, result *model.DeviceList) {
				require.NotNil(t, result)
				require.Equal(t, uint(1), result.Pagination.Page)
			},
		},
		{
			name: "list with brand and state filters",
			filter: model.DeviceFilter{
				Brand: &brand,
				State: &state,
				Page:  1,
				Size:  10,
			},
			setupSvc: func() *mocks.FakeDevicesService {
				fake := &mocks.FakeDevicesService{}
				fake.ListDevicesStub = func(_ context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
					return &model.DeviceList{
						Devices: []*model.Device{},
						Pagination: model.Pagination{
							Page:       filter.Page,
							Size:       filter.Size,
							TotalItems: 0,
							TotalPages: 1,
						},
						Filters: filter,
					}, nil
				}

				return fake
			},
			validate: func(t *testing.T, result *model.DeviceList) {
				require.NotNil(t, result)
				require.Equal(t, &brand, result.Filters.Brand)
				require.Equal(t, &state, result.Filters.State)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := tc.setupSvc()

			handler := queries.NewListDevicesQueryHandler(svc, log, mc, tp)
			query := queries.ListDevicesQuery{Filter: tc.filter}

			result, err := handler.Execute(t.Context(), query)

			require.NoError(t, err)
			tc.validate(t, result)
		})
	}
}

func TestFetchLivenessQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	mc := noop.NewMetricsClient()
	tp := otelNoop.NewTracerProvider()

	cases := []struct {
		name     string
		setupSvc func() *mocks.FakeHealthChecker
		validate func(*testing.T, *model.LivenessReport)
	}{
		{
			name: "healthy liveness",
			setupSvc: func() *mocks.FakeHealthChecker {
				fake := &mocks.FakeHealthChecker{}
				fake.LivenessReturns(&model.LivenessReport{
					Status:    model.HealthStatusOK,
					Timestamp: time.Now().UTC(),
					Version:   "1.0.0",
				}, nil)

				return fake
			},
			validate: func(t *testing.T, result *model.LivenessReport) {
				require.NotNil(t, result)
				require.Equal(t, model.HealthStatusOK, result.Status)
				require.NotEmpty(t, result.Version)
				require.False(t, result.Timestamp.IsZero())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			healthChecker := tc.setupSvc()

			handler := queries.NewFetchLivenessQueryHandler(healthChecker, log, mc, tp)
			query := queries.FetchLivenessQuery{}

			result, err := handler.Execute(t.Context(), query)

			require.NoError(t, err)
			tc.validate(t, result)
		})
	}
}

func TestFetchReadinessQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	mc := noop.NewMetricsClient()
	tp := otelNoop.NewTracerProvider()

	cases := []struct {
		name     string
		setupSvc func() *mocks.FakeHealthChecker
		validate func(*testing.T, *model.ReadinessReport)
	}{
		{
			name: "healthy readiness",
			setupSvc: func() *mocks.FakeHealthChecker {
				fake := &mocks.FakeHealthChecker{}
				fake.ReadinessReturns(&model.ReadinessReport{
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
				}, nil)

				return fake
			},
			validate: func(t *testing.T, result *model.ReadinessReport) {
				require.NotNil(t, result)
				require.Equal(t, model.HealthStatusOK, result.Status)
				require.Contains(t, result.Checks, "storage")
				require.Equal(t, model.DependencyStatusUp, result.Checks["storage"].Status)
			},
		},
		{
			name: "unhealthy readiness",
			setupSvc: func() *mocks.FakeHealthChecker {
				fake := &mocks.FakeHealthChecker{}
				fake.ReadinessReturns(&model.ReadinessReport{
					Status:    model.HealthStatusDown,
					Timestamp: time.Now().UTC(),
					Version:   "1.0.0",
					Checks: map[string]model.DependencyCheck{
						"storage": {
							Status:      model.DependencyStatusDown,
							LatencyMs:   0,
							Message:     "connection refused",
							LastChecked: time.Now().UTC(),
							Error:       "connection refused",
						},
					},
				}, nil)

				return fake
			},
			validate: func(t *testing.T, result *model.ReadinessReport) {
				require.NotNil(t, result)
				require.Equal(t, model.HealthStatusDown, result.Status)
				require.Contains(t, result.Checks, "storage")
				require.Equal(t, model.DependencyStatusDown, result.Checks["storage"].Status)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			healthChecker := tc.setupSvc()

			handler := queries.NewFetchReadinessQueryHandler(healthChecker, log, mc, tp)
			query := queries.FetchReadinessQuery{}

			result, err := handler.Execute(t.Context(), query)

			require.NoError(t, err)
			tc.validate(t, result)
		})
	}
}

func TestFetchHealthReportQueryHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	mc := noop.NewMetricsClient()
	tp := otelNoop.NewTracerProvider()

	cases := []struct {
		name     string
		setupSvc func() *mocks.FakeHealthChecker
		validate func(*testing.T, *model.HealthReport)
	}{
		{
			name: "healthy report",
			setupSvc: func() *mocks.FakeHealthChecker {
				fake := &mocks.FakeHealthChecker{}
				fake.HealthReturns(&model.HealthReport{
					Status:    model.HealthStatusOK,
					Timestamp: time.Now().UTC(),
					Version: model.VersionInfo{
						API:   "1.0.0",
						Build: "development",
						Go:    "1.25",
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
				}, nil)

				return fake
			},
			validate: func(t *testing.T, result *model.HealthReport) {
				require.NotNil(t, result)
				require.Equal(t, model.HealthStatusOK, result.Status)
				require.NotEmpty(t, result.Version.API)
				require.NotEmpty(t, result.Version.Go)
				require.Greater(t, result.System.CPUCores, uint(0))
				require.Greater(t, result.System.Goroutines, uint(0))
			},
		},
		{
			name: "unhealthy report",
			setupSvc: func() *mocks.FakeHealthChecker {
				fake := &mocks.FakeHealthChecker{}
				fake.HealthReturns(&model.HealthReport{
					Status:    model.HealthStatusDown,
					Timestamp: time.Now().UTC(),
					Version: model.VersionInfo{
						API:   "1.0.0",
						Build: "development",
						Go:    "1.25",
					},
					Checks: map[string]model.DependencyCheck{
						"storage": {
							Status:      model.DependencyStatusDown,
							LatencyMs:   0,
							Message:     "service unavailable",
							LastChecked: time.Now().UTC(),
							Error:       "service unavailable",
						},
					},
					System: model.SystemInfo{
						Goroutines: 1,
						CPUCores:   1,
					},
				}, nil)

				return fake
			},
			validate: func(t *testing.T, result *model.HealthReport) {
				require.NotNil(t, result)
				require.Equal(t, model.HealthStatusDown, result.Status)
				require.Contains(t, result.Checks, "storage")
				require.Equal(t, model.DependencyStatusDown, result.Checks["storage"].Status)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			healthChecker := tc.setupSvc()

			handler := queries.NewFetchHealthReportQueryHandler(healthChecker, log, mc, tp)
			query := queries.FetchHealthReportQuery{}

			result, err := handler.Execute(t.Context(), query)

			require.NoError(t, err)
			tc.validate(t, result)
		})
	}
}
