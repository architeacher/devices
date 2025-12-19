package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
	"github.com/stretchr/testify/suite"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

type mockDeviceService struct {
	getDeviceFn   func(ctx context.Context, id model.DeviceID) (*model.Device, error)
	listDevicesFn func(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
}

func (m *mockDeviceService) CreateDevice(_ context.Context, name, brand string, state model.State) (*model.Device, error) {
	return model.NewDevice(name, brand, state), nil
}

func (m *mockDeviceService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}

	return &model.Device{ID: id}, nil
}

func (m *mockDeviceService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	if m.listDevicesFn != nil {
		return m.listDevicesFn(ctx, filter)
	}

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

func (m *mockDeviceService) UpdateDevice(_ context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	return &model.Device{ID: id, Name: name, Brand: brand, State: state}, nil
}

func (m *mockDeviceService) PatchDevice(_ context.Context, id model.DeviceID, _ map[string]any) (*model.Device, error) {
	return &model.Device{ID: id}, nil
}

func (m *mockDeviceService) DeleteDevice(_ context.Context, _ model.DeviceID) error {
	return nil
}

type mockHealthChecker struct {
	livenessFn  func(ctx context.Context) (*model.LivenessReport, error)
	readinessFn func(ctx context.Context) (*model.ReadinessReport, error)
	healthFn    func(ctx context.Context) (*model.HealthReport, error)
}

func (m *mockHealthChecker) Liveness(ctx context.Context) (*model.LivenessReport, error) {
	if m.livenessFn != nil {
		return m.livenessFn(ctx)
	}

	return &model.LivenessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}, nil
}

func (m *mockHealthChecker) Readiness(ctx context.Context) (*model.ReadinessReport, error) {
	if m.readinessFn != nil {
		return m.readinessFn(ctx)
	}

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

func (m *mockHealthChecker) Health(ctx context.Context) (*model.HealthReport, error) {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}

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

type GetDeviceQueryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestGetDeviceQueryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GetDeviceQueryTestSuite))
}

func (s *GetDeviceQueryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *GetDeviceQueryTestSuite) TestExecute_Success() {
	s.T().Parallel()

	id := model.NewDeviceID()
	expectedDevice := &model.Device{
		ID:    id,
		Name:  "Test Device",
		Brand: "Test Brand",
		State: model.StateAvailable,
	}

	svc := &mockDeviceService{
		getDeviceFn: func(_ context.Context, _ model.DeviceID) (*model.Device, error) {
			return expectedDevice, nil
		},
	}
	handler := queries.NewGetDeviceQueryHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.GetDeviceQuery{ID: id}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(expectedDevice.ID, result.ID)
	s.Require().Equal(expectedDevice.Name, result.Name)
}

func (s *GetDeviceQueryTestSuite) TestExecute_NotFound() {
	s.T().Parallel()

	svc := &mockDeviceService{
		getDeviceFn: func(_ context.Context, _ model.DeviceID) (*model.Device, error) {
			return nil, model.ErrDeviceNotFound
		},
	}
	handler := queries.NewGetDeviceQueryHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.GetDeviceQuery{ID: model.NewDeviceID()}

	result, err := handler.Execute(s.ctx, query)

	s.Require().ErrorIs(err, model.ErrDeviceNotFound)
	s.Require().Nil(result)
}

type ListDevicesQueryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestListDevicesQueryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ListDevicesQueryTestSuite))
}

func (s *ListDevicesQueryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ListDevicesQueryTestSuite) TestExecute_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	handler := queries.NewListDevicesQueryHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	filter := model.DefaultDeviceFilter()
	query := queries.ListDevicesQuery{Filter: filter}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(filter.Page, result.Pagination.Page)
	s.Require().Equal(filter.Size, result.Pagination.Size)
}

func (s *ListDevicesQueryTestSuite) TestExecute_WithFilters() {
	s.T().Parallel()

	brand := "Apple"
	state := model.StateAvailable

	svc := &mockDeviceService{
		listDevicesFn: func(_ context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
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
		},
	}
	handler := queries.NewListDevicesQueryHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	filter := model.DeviceFilter{
		Brand: &brand,
		State: &state,
		Page:  1,
		Size:  10,
	}
	query := queries.ListDevicesQuery{Filter: filter}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(&brand, result.Filters.Brand)
	s.Require().Equal(&state, result.Filters.State)
}

type FetchLivenessQueryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFetchLivenessQueryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FetchLivenessQueryTestSuite))
}

func (s *FetchLivenessQueryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FetchLivenessQueryTestSuite) TestExecute_Success() {
	s.T().Parallel()

	healthChecker := &mockHealthChecker{}
	handler := queries.NewFetchLivenessQueryHandler(healthChecker, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.FetchLivenessQuery{}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(model.HealthStatusOK, result.Status)
	s.Require().NotEmpty(result.Version)
	s.Require().False(result.Timestamp.IsZero())
}

type FetchReadinessQueryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFetchReadinessQueryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FetchReadinessQueryTestSuite))
}

func (s *FetchReadinessQueryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FetchReadinessQueryTestSuite) TestExecute_Healthy() {
	s.T().Parallel()

	healthChecker := &mockHealthChecker{}
	handler := queries.NewFetchReadinessQueryHandler(healthChecker, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.FetchReadinessQuery{}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(model.HealthStatusOK, result.Status)
	s.Require().Contains(result.Checks, "storage")
	s.Require().Equal(model.DependencyStatusUp, result.Checks["storage"].Status)
}

func (s *FetchReadinessQueryTestSuite) TestExecute_Unhealthy() {
	s.T().Parallel()

	healthChecker := &mockHealthChecker{
		readinessFn: func(_ context.Context) (*model.ReadinessReport, error) {
			return &model.ReadinessReport{
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
			}, nil
		},
	}
	handler := queries.NewFetchReadinessQueryHandler(healthChecker, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.FetchReadinessQuery{}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(model.HealthStatusDown, result.Status)
	s.Require().Contains(result.Checks, "storage")
	s.Require().Equal(model.DependencyStatusDown, result.Checks["storage"].Status)
}

type FetchHealthReportQueryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFetchHealthReportQueryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FetchHealthReportQueryTestSuite))
}

func (s *FetchHealthReportQueryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FetchHealthReportQueryTestSuite) TestExecute_Healthy() {
	s.T().Parallel()

	healthChecker := &mockHealthChecker{}
	handler := queries.NewFetchHealthReportQueryHandler(healthChecker, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.FetchHealthReportQuery{}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(model.HealthStatusOK, result.Status)
	s.Require().NotEmpty(result.Version.API)
	s.Require().NotEmpty(result.Version.Go)
	s.Require().Greater(result.System.CPUCores, uint(0))
	s.Require().Greater(result.System.Goroutines, uint(0))
}

func (s *FetchHealthReportQueryTestSuite) TestExecute_Unhealthy() {
	s.T().Parallel()

	healthChecker := &mockHealthChecker{
		healthFn: func(_ context.Context) (*model.HealthReport, error) {
			return &model.HealthReport{
				Status:    model.HealthStatusDown,
				Timestamp: time.Now().UTC(),
				Version: model.VersionInfo{
					API:   "1.0.0",
					Build: "development",
					Go:    "1.23",
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
			}, nil
		},
	}
	handler := queries.NewFetchHealthReportQueryHandler(healthChecker, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	query := queries.FetchHealthReportQuery{}

	result, err := handler.Execute(s.ctx, query)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(model.HealthStatusDown, result.Status)
	s.Require().Contains(result.Checks, "storage")
	s.Require().Equal(model.DependencyStatusDown, result.Checks["storage"].Status)
}
