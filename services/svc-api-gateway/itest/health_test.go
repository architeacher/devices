//go:build integration

package itest

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HealthCheckTestSuite struct {
	suite.Suite
	server *IntegrationTestServer
}

func TestHealthCheckTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(HealthCheckTestSuite))
}

func (s *HealthCheckTestSuite) SetupSuite() {
	ctx := context.Background()
	server, err := NewIntegrationTestServer(ctx)
	s.Require().NoError(err)
	s.server = server
}

func (s *HealthCheckTestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *HealthCheckTestSuite) getHealth(path string) (map[string]any, *http.Response) {
	resp, err := s.server.Get(s.T().Context(), path)
	s.Require().NoError(err)

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))

	return response, resp
}

func (s *HealthCheckTestSuite) TestLivenessCheck() {
	response, resp := s.getHealth("/v1/liveness")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Equal("ok", response["status"])
	s.Require().NotEmpty(response["timestamp"])
}

func (s *HealthCheckTestSuite) TestReadinessCheck() {
	response, resp := s.getHealth("/v1/readiness")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Equal("ok", response["status"])
	s.Require().NotEmpty(response["timestamp"])
}

func (s *HealthCheckTestSuite) TestHealthCheck() {
	response, resp := s.getHealth("/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Equal("ok", response["status"])
	s.Require().NotEmpty(response["timestamp"])
}

func (s *HealthCheckTestSuite) TestHealthCheckReturnsVersionInfo() {
	response, resp := s.getHealth("/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	version, ok := response["version"].(map[string]any)
	s.Require().True(ok, "version should be a map")
	s.Require().NotNil(version["go"], "go version should be present")
}

func (s *HealthCheckTestSuite) TestHealthCheckReturnsUptimeInfo() {
	response, resp := s.getHealth("/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	uptime := response["uptime"].(map[string]any)
	s.Require().NotEmpty(uptime["duration"])
	s.Require().NotNil(uptime["durationSeconds"])
	s.Require().NotEmpty(uptime["startedAt"])
}

func (s *HealthCheckTestSuite) TestHealthCheckReturnsSystemInfo() {
	response, resp := s.getHealth("/v1/health")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	system := response["system"].(map[string]any)
	s.Require().NotNil(system["cpuCores"])
	s.Require().NotNil(system["goroutines"])
}

func (s *HealthCheckTestSuite) TestLivenessReturnsVersionAsString() {
	response, resp := s.getHealth("/v1/liveness")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	_, ok := response["version"].(string)
	s.Require().True(ok, "version should be a string in liveness response")
}

func (s *HealthCheckTestSuite) TestHealthEndpointsReturnJSON() {
	endpoints := []string{"/v1/liveness", "/v1/readiness", "/v1/health"}

	for _, endpoint := range endpoints {
		s.Run(endpoint, func() {
			resp, err := s.server.Get(s.T().Context(), endpoint)
			s.Require().NoError(err)
			defer resp.Body.Close()

			contentType := resp.Header.Get("Content-Type")
			s.Require().Contains(contentType, "application/json")
		})
	}
}
