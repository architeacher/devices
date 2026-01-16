package admin_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers/admin"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/suite"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

type AdminHandlerTestSuite struct {
	suite.Suite
}

func TestAdminHandlerTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AdminHandlerTestSuite))
}

func newTestApp(healthChecker *mocks.FakeHealthChecker) *usecases.WebApplication {
	deviceSvc := &mocks.FakeDevicesService{}

	return usecases.NewWebApplication(
		deviceSvc,
		healthChecker,
		nil,
		logger.NewTestLogger(),
		noop.NewMetricsClient(),
		otelNoop.NewTracerProvider(),
	)
}

func newDefaultHealthChecker() *mocks.FakeHealthChecker {
	hc := &mocks.FakeHealthChecker{}
	hc.LivenessReturns(&model.LivenessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}, nil)
	hc.ReadinessReturns(&model.ReadinessReport{
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
	hc.HealthReturns(&model.HealthReport{
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

	return hc
}

func (s *AdminHandlerTestSuite) TestGetCacheHealth_Healthy() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.IsHealthyReturns(true)
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodGet, "/admin/cache/health", nil)
	rec := httptest.NewRecorder()

	handler.GetCacheHealth(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal("healthy", response["status"])
	s.Require().Equal(1, cache.IsHealthyCallCount())
}

func (s *AdminHandlerTestSuite) TestGetCacheHealth_Unhealthy() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.IsHealthyReturns(false)
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodGet, "/admin/cache/health", nil)
	rec := httptest.NewRecorder()

	handler.GetCacheHealth(rec, req)

	s.Require().Equal(http.StatusServiceUnavailable, rec.Code)
	s.Require().Equal(1, cache.IsHealthyCallCount())
}

func (s *AdminHandlerTestSuite) TestGetCacheHealth_NilCache() {
	s.T().Parallel()

	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(nil, app)

	req := httptest.NewRequest(http.MethodGet, "/admin/cache/health", nil)
	rec := httptest.NewRecorder()

	handler.GetCacheHealth(rec, req)

	s.Require().Equal(http.StatusServiceUnavailable, rec.Code)
}

func (s *AdminHandlerTestSuite) TestPurgeAllDeviceCaches_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices", nil)
	rec := httptest.NewRecorder()

	handler.PurgeAllDeviceCaches(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Contains(response["status"], "purged")
	s.Require().Equal(1, cache.PurgeAllCallCount())
}

func (s *AdminHandlerTestSuite) TestPurgeAllDeviceCaches_NilCache() {
	s.T().Parallel()

	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(nil, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices", nil)
	rec := httptest.NewRecorder()

	handler.PurgeAllDeviceCaches(rec, req)

	s.Require().Equal(http.StatusServiceUnavailable, rec.Code)
}

func (s *AdminHandlerTestSuite) TestPurgeAllDeviceCaches_Error() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.PurgeAllReturns(errors.New("purge failed"))
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices", nil)
	rec := httptest.NewRecorder()

	handler.PurgeAllDeviceCaches(rec, req)

	s.Require().Equal(http.StatusInternalServerError, rec.Code)
	s.Require().Equal(1, cache.PurgeAllCallCount())
}

func (s *AdminHandlerTestSuite) TestPurgeDeviceCache_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)
	deviceID := model.NewDeviceID()

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices/"+deviceID.String(), nil)
	rec := httptest.NewRecorder()

	handler.PurgeDeviceCache(rec, req, openapi_types.UUID(deviceID.UUID))

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Require().Equal(1, cache.InvalidateDeviceCallCount())
}

func (s *AdminHandlerTestSuite) TestPurgeDeviceCache_NilCache() {
	s.T().Parallel()

	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(nil, app)
	deviceID := model.NewDeviceID()

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices/"+deviceID.String(), nil)
	rec := httptest.NewRecorder()

	handler.PurgeDeviceCache(rec, req, openapi_types.UUID(deviceID.UUID))

	s.Require().Equal(http.StatusServiceUnavailable, rec.Code)
}

func (s *AdminHandlerTestSuite) TestPurgeDeviceCache_Error() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.InvalidateDeviceReturns(errors.New("invalidate failed"))
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)
	deviceID := model.NewDeviceID()

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices/"+deviceID.String(), nil)
	rec := httptest.NewRecorder()

	handler.PurgeDeviceCache(rec, req, openapi_types.UUID(deviceID.UUID))

	s.Require().Equal(http.StatusInternalServerError, rec.Code)
	s.Require().Equal(1, cache.InvalidateDeviceCallCount())
}

func (s *AdminHandlerTestSuite) TestPurgeDeviceListCaches_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices/lists", nil)
	rec := httptest.NewRecorder()

	handler.PurgeDeviceListCaches(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Require().Equal(1, cache.InvalidateAllListsCallCount())
}

func (s *AdminHandlerTestSuite) TestPurgeDeviceListCaches_NilCache() {
	s.T().Parallel()

	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(nil, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices/lists", nil)
	rec := httptest.NewRecorder()

	handler.PurgeDeviceListCaches(rec, req)

	s.Require().Equal(http.StatusServiceUnavailable, rec.Code)
}

func (s *AdminHandlerTestSuite) TestPurgeDeviceListCaches_Error() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.InvalidateAllListsReturns(errors.New("invalidate failed"))
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/devices/lists", nil)
	rec := httptest.NewRecorder()

	handler.PurgeDeviceListCaches(rec, req)

	s.Require().Equal(http.StatusInternalServerError, rec.Code)
	s.Require().Equal(1, cache.InvalidateAllListsCallCount())
}

func (s *AdminHandlerTestSuite) TestPurgeCacheByPattern_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.PurgeByPatternReturns(5, nil)
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/pattern?pattern=device:*", nil)
	rec := httptest.NewRecorder()

	handler.PurgeCacheByPattern(rec, req, admin.PurgeCacheByPatternParams{
		Pattern: "device:*",
	})

	s.Require().Equal(http.StatusOK, rec.Code)

	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal(float64(5), response["deleted"])
	s.Require().Equal(1, cache.PurgeByPatternCallCount())

	_, pattern := cache.PurgeByPatternArgsForCall(0)
	s.Require().Equal("device:*", pattern)
}

func (s *AdminHandlerTestSuite) TestPurgeCacheByPattern_NilCache() {
	s.T().Parallel()

	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(nil, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/pattern?pattern=device:*", nil)
	rec := httptest.NewRecorder()

	handler.PurgeCacheByPattern(rec, req, admin.PurgeCacheByPatternParams{
		Pattern: "device:*",
	})

	s.Require().Equal(http.StatusServiceUnavailable, rec.Code)
}

func (s *AdminHandlerTestSuite) TestPurgeCacheByPattern_Error() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.PurgeByPatternReturns(0, errors.New("purge failed"))
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodDelete, "/admin/cache/pattern?pattern=device:*", nil)
	rec := httptest.NewRecorder()

	handler.PurgeCacheByPattern(rec, req, admin.PurgeCacheByPatternParams{
		Pattern: "device:*",
	})

	s.Require().Equal(http.StatusInternalServerError, rec.Code)
	s.Require().Equal(1, cache.PurgeByPatternCallCount())
}

func (s *AdminHandlerTestSuite) TestLivenessCheck_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodGet, "/liveness", nil)
	rec := httptest.NewRecorder()

	handler.LivenessCheck(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)

	var response admin.Liveness
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal(admin.LivenessStatusOk, response.Status)
}

func (s *AdminHandlerTestSuite) TestReadinessCheck_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()

	handler.ReadinessCheck(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)

	var response admin.Readiness
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal(admin.Ok, response.Status)
}

func (s *AdminHandlerTestSuite) TestHealthCheck_Success() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	app := newTestApp(newDefaultHealthChecker())
	handler := admin.NewAdminHandler(cache, app)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.HealthCheck(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)

	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal("ok", response["status"])
}
