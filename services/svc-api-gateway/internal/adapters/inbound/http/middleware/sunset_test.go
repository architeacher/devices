package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/stretchr/testify/suite"
)

type SunsetMiddlewareSuite struct {
	suite.Suite
}

func TestSunsetMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(SunsetMiddlewareSuite))
}

func (s *SunsetMiddlewareSuite) TestSunset_Disabled() {
	s.T().Parallel()

	cfg := config.Deprecation{
		Enabled: false,
	}

	handler := middleware.Sunset(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Require().Empty(rec.Header().Get("Deprecation"))
	s.Require().Empty(rec.Header().Get("Sunset"))
	s.Require().Empty(rec.Header().Get("Link"))
}

func (s *SunsetMiddlewareSuite) TestSunset_EnabledWithAllHeaders() {
	s.T().Parallel()

	sunsetDate := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
	cfg := config.Deprecation{
		Enabled:       true,
		SunsetDate:    sunsetDate.Format(time.RFC3339),
		SuccessorPath: "/v2/devices",
	}

	handler := middleware.Sunset(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Require().Equal("true", rec.Header().Get("Deprecation"))
	s.Require().Equal("Wed, 31 Dec 2025 23:59:59 GMT", rec.Header().Get("Sunset"))
	s.Require().Equal("</v2/devices>; rel=\"successor-version\"", rec.Header().Get("Link"))
}

func (s *SunsetMiddlewareSuite) TestSunset_EnabledWithoutSuccessorPath() {
	s.T().Parallel()

	sunsetDate := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)
	cfg := config.Deprecation{
		Enabled:       true,
		SunsetDate:    sunsetDate.Format(time.RFC3339),
		SuccessorPath: "",
	}

	handler := middleware.Sunset(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Require().Equal("true", rec.Header().Get("Deprecation"))
	s.Require().Equal("Mon, 30 Jun 2025 00:00:00 GMT", rec.Header().Get("Sunset"))
	s.Require().Empty(rec.Header().Get("Link"))
}

func (s *SunsetMiddlewareSuite) TestSunset_EnabledWithInvalidSunsetDate() {
	s.T().Parallel()

	cfg := config.Deprecation{
		Enabled:       true,
		SunsetDate:    "invalid-date",
		SuccessorPath: "/v2/devices",
	}

	handler := middleware.Sunset(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Require().Equal("true", rec.Header().Get("Deprecation"))
	s.Require().Empty(rec.Header().Get("Sunset"))
	s.Require().Equal("</v2/devices>; rel=\"successor-version\"", rec.Header().Get("Link"))
}
