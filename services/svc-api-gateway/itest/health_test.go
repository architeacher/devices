package itest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type HealthCheckTestSuite struct {
	suite.Suite
}

func TestHealthCheckTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(HealthCheckTestSuite))
}

func (s *HealthCheckTestSuite) getHealth(server *TestServer, path string) (map[string]any, *http.Response) {
	resp, err := server.Get(s.T().Context(), path)
	s.Require().NoError(err)

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))

	return response, resp
}

func (s *HealthCheckTestSuite) TestLivenessCheck() {
	cases := []struct {
		name           string
		setupFn        func(server *TestServer)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "liveness returns ok",
			setupFn:        func(_ *TestServer) {},
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name: "liveness returns down on error",
			setupFn: func(server *TestServer) {
				server.HealthChecker.LivenessFn = func(_ context.Context) (*model.LivenessReport, error) {
					return nil, errors.New("liveness check failed")
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "down",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			tc.setupFn(server)

			response, resp := s.getHealth(server, "/v1/liveness")
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
			s.Require().Equal(tc.expectedBody, response["status"])
			s.Require().NotEmpty(response["timestamp"])
		})
	}
}

func (s *HealthCheckTestSuite) TestReadinessCheck() {
	cases := []struct {
		name           string
		setupFn        func(server *TestServer)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "readiness returns ok",
			setupFn:        func(_ *TestServer) {},
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name: "readiness returns down on error",
			setupFn: func(server *TestServer) {
				server.HealthChecker.ReadinessFn = func(_ context.Context) (*model.ReadinessReport, error) {
					return nil, errors.New("readiness check failed")
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "down",
		},
		{
			name: "readiness returns down when status is not ok",
			setupFn: func(server *TestServer) {
				server.HealthChecker.ReadinessFn = func(_ context.Context) (*model.ReadinessReport, error) {
					return &model.ReadinessReport{
						Status:    model.HealthStatusDown,
						Timestamp: time.Now().UTC(),
					}, nil
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "down",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			tc.setupFn(server)

			response, resp := s.getHealth(server, "/v1/readiness")
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
			s.Require().Equal(tc.expectedBody, response["status"])
			s.Require().NotEmpty(response["timestamp"])
		})
	}
}

func (s *HealthCheckTestSuite) TestHealthCheck() {
	cases := []struct {
		name           string
		setupFn        func(server *TestServer)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "health returns ok",
			setupFn:        func(_ *TestServer) {},
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name: "health returns down on error",
			setupFn: func(server *TestServer) {
				server.HealthChecker.HealthFn = func(_ context.Context) (*model.HealthReport, error) {
					return nil, errors.New("health check failed")
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "down",
		},
		{
			name: "health returns degraded status as down",
			setupFn: func(server *TestServer) {
				server.HealthChecker.HealthFn = func(_ context.Context) (*model.HealthReport, error) {
					return &model.HealthReport{
						Status:    model.HealthStatusDegraded,
						Timestamp: time.Now().UTC(),
						Version: model.VersionInfo{
							API:   "1.0.0",
							Build: "test",
							Go:    "1.25",
						},
						System: model.SystemInfo{
							Goroutines: 10,
							CPUCores:   4,
						},
					}, nil
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "down",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			tc.setupFn(server)

			response, resp := s.getHealth(server, "/v1/health")
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
			s.Require().Equal(tc.expectedBody, response["status"])
			s.Require().NotEmpty(response["timestamp"])
		})
	}
}

func (s *HealthCheckTestSuite) TestHealthCheckReturnsVersionInfo() {
	server := NewTestServer()
	defer server.Close()

	response, resp := s.getHealth(server, "/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	version := response["version"].(map[string]any)
	s.Require().NotEmpty(version["api"])
	s.Require().NotEmpty(version["build"])
	s.Require().NotEmpty(version["go"])
}

func (s *HealthCheckTestSuite) TestHealthCheckReturnsUptimeInfo() {
	server := NewTestServer()
	defer server.Close()

	response, resp := s.getHealth(server, "/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	uptime := response["uptime"].(map[string]any)
	s.Require().NotEmpty(uptime["duration"])
	s.Require().NotNil(uptime["durationSeconds"])
	s.Require().NotEmpty(uptime["startedAt"])
}

func (s *HealthCheckTestSuite) TestHealthCheckReturnsSystemInfo() {
	server := NewTestServer()
	defer server.Close()

	response, resp := s.getHealth(server, "/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	system := response["system"].(map[string]any)
	s.Require().NotNil(system["cpuCores"])
	s.Require().NotNil(system["goroutines"])
}

func (s *HealthCheckTestSuite) TestLivenessReturnsVersion() {
	server := NewTestServer()
	defer server.Close()

	response, resp := s.getHealth(server, "/v1/liveness")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().NotEmpty(response["version"])
}

func (s *HealthCheckTestSuite) TestHealthEndpointsReturnJSON() {
	endpoints := []string{"/v1/liveness", "/v1/readiness", "/v1/health"}

	for _, endpoint := range endpoints {
		s.Run(endpoint, func() {
			server := NewTestServer()
			defer server.Close()

			resp, err := server.Get(s.T().Context(), endpoint)
			s.Require().NoError(err)
			defer resp.Body.Close()

			contentType := resp.Header.Get("Content-Type")
			s.Require().Contains(contentType, "application/json")
		})
	}
}
