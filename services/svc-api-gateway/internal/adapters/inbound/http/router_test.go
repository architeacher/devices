package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	inboundhttp "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/stretchr/testify/suite"
)

type AdminRouterTestSuite struct {
	suite.Suite
}

func TestAdminRouterTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AdminRouterTestSuite))
}

func (s *AdminRouterTestSuite) TestNewAdminRouter_RoutesRegistered() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	cache.IsHealthyReturns(true)
	log := logger.New("debug", "console")

	router := inboundhttp.NewAdminRouter(inboundhttp.AdminRouterConfig{
		DevicesCache: cache,
		Logger:       log,
	})

	deviceID := model.NewDeviceID()

	cases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		setupCache     func()
	}{
		{
			name:           "GET /admin/cache/health returns healthy status",
			method:         http.MethodGet,
			path:           "/admin/cache/health",
			expectedStatus: http.StatusOK,
			setupCache: func() {
				cache.IsHealthyReturns(true)
			},
		},
		{
			name:           "DELETE /admin/cache/devices purges all caches",
			method:         http.MethodDelete,
			path:           "/admin/cache/devices",
			expectedStatus: http.StatusOK,
			setupCache:     func() {},
		},
		{
			name:           "DELETE /admin/cache/devices/{id} purges specific device",
			method:         http.MethodDelete,
			path:           "/admin/cache/devices/" + deviceID.String(),
			expectedStatus: http.StatusOK,
			setupCache:     func() {},
		},
		{
			name:           "DELETE /admin/cache/devices/lists purges list caches",
			method:         http.MethodDelete,
			path:           "/admin/cache/devices/lists",
			expectedStatus: http.StatusOK,
			setupCache:     func() {},
		},
		{
			name:           "DELETE /admin/cache/pattern purges by pattern",
			method:         http.MethodDelete,
			path:           "/admin/cache/pattern?pattern=device:*",
			expectedStatus: http.StatusOK,
			setupCache:     func() {},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.setupCache()

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			s.Require().Equal(tc.expectedStatus, rec.Code, "unexpected status for %s %s", tc.method, tc.path)
		})
	}
}

func (s *AdminRouterTestSuite) TestNewAdminRouter_NilCache_Returns503() {
	s.T().Parallel()

	log := logger.New("debug", "console")

	router := inboundhttp.NewAdminRouter(inboundhttp.AdminRouterConfig{
		DevicesCache: nil,
		Logger:       log,
	})

	cases := []struct {
		name          string
		method        string
		path          string
		expectedError string
	}{
		{
			name:          "health check with nil cache",
			method:        http.MethodGet,
			path:          "/admin/cache/health",
			expectedError: "cache not configured",
		},
		{
			name:          "purge all with nil cache",
			method:        http.MethodDelete,
			path:          "/admin/cache/devices",
			expectedError: "cache not available",
		},
		{
			name:          "purge lists with nil cache",
			method:        http.MethodDelete,
			path:          "/admin/cache/devices/lists",
			expectedError: "cache not available",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			s.Require().Equal(http.StatusServiceUnavailable, rec.Code)

			var response map[string]string
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			s.Require().NoError(err)
			s.Require().Contains(response["error"], tc.expectedError)
		})
	}
}

func (s *AdminRouterTestSuite) TestNewAdminRouter_UnknownRoute_Returns404() {
	s.T().Parallel()

	cache := &mocks.FakeDevicesCache{}
	log := logger.New("debug", "console")

	router := inboundhttp.NewAdminRouter(inboundhttp.AdminRouterConfig{
		DevicesCache: cache,
		Logger:       log,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/unknown", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusNotFound, rec.Code)
}
