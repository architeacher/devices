//go:build integration

package itest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	internalhttp "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http"
	grpcclient "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/outbound/grpc"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/services"
	apiconfig "github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/architeacher/devices/services/svc-devices/testutil"
	"github.com/stretchr/testify/suite"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BaseTestSuite provides common setup and helper methods for integration tests.
type BaseTestSuite struct {
	suite.Suite
	Server *IntegrationTestServer
}

// SetupSuite initializes the integration test server.
func (s *BaseTestSuite) SetupSuite() {
	server, err := NewIntegrationTestServer(s.T().Context())
	s.Require().NoError(err)
	s.Server = server
}

// TearDownSuite shuts down the integration test server.
func (s *BaseTestSuite) TearDownSuite() {
	if s.Server != nil {
		s.Server.Close()
	}
}

// SetupTest truncates devices before each test.
func (s *BaseTestSuite) SetupTest() {
	err := s.Server.TruncateDevices(s.T().Context())
	s.Require().NoError(err)
}

// CreateDevice creates a device and returns its ID.
func (s *BaseTestSuite) CreateDevice(name, brand string, state model.State) string {
	body := map[string]any{
		"name":  name,
		"brand": brand,
		"state": string(state),
	}
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)

	resp, err := s.Server.Post(s.T().Context(), "/v1/devices", bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var response map[string]any
	err = DecodeJSON(resp.Body, &response)
	s.Require().NoError(err)

	data := response["data"].(map[string]any)

	return data["id"].(string)
}

// DevicePath returns the API path for a device by ID.
func (s *BaseTestSuite) DevicePath(id string) string {
	return fmt.Sprintf("/v1/devices/%s", id)
}

// IntegrationTestServer provides a full integration test environment with
// PostgreSQL, gRPC svc-devices, and HTTP svc-api-gateway.
type IntegrationTestServer struct {
	HTTPServer *httptest.Server
	TestServer *testutil.TestServer
	GRPCClient *services.DevicesService
	grpcConn   *grpc.ClientConn
}

// NewIntegrationTestServer creates a complete integration test environment.
func NewIntegrationTestServer(ctx context.Context) (*IntegrationTestServer, error) {
	migrationsPath, err := getMigrationsPath()
	if err != nil {
		return nil, fmt.Errorf("getting migrations path: %w", err)
	}

	testServer, err := testutil.New(ctx, testutil.WithMigrationsPath(migrationsPath))
	if err != nil {
		return nil, err
	}

	grpcAddress := testServer.Address()

	conn, err := grpc.NewClient(grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		testServer.Close()

		return nil, err
	}

	grpcOutbound, err := grpcclient.NewClient(
		&apiconfig.ServiceConfig{
			DevicesGRPCClient: apiconfig.DevicesGRPCClient{
				Address:        grpcAddress,
				Timeout:        10 * time.Second,
				MaxRetries:     1,
				CircuitBreaker: apiconfig.CircuitBreakerConfig{Enabled: false},
			},
		},
		grpcclient.WithConnection(conn),
	)
	if err != nil {
		conn.Close()
		testServer.Close()

		return nil, err
	}

	grpcClient, err := services.NewDevicesService(
		&apiconfig.ServiceConfig{
			DevicesGRPCClient: apiconfig.DevicesGRPCClient{
				Address:        grpcAddress,
				Timeout:        10 * time.Second,
				MaxRetries:     1,
				CircuitBreaker: apiconfig.CircuitBreakerConfig{Enabled: false},
			},
		},
		services.WithConnection(grpcOutbound),
	)
	if err != nil {
		grpcOutbound.Close()
		conn.Close()
		testServer.Close()

		return nil, err
	}

	log := logger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()
	tracerProvider := otelNoop.NewTracerProvider()

	apiApp := usecases.NewWebApplication(grpcClient, grpcClient, nil, log, metricsClient, tracerProvider)

	cfg := &apiconfig.ServiceConfig{
		App: apiconfig.App{
			ServiceName: "api-gateway-test",
			APIVersion:  "v1",
			Env:         apiconfig.Environment{Name: "test"},
		},
		PublicHTTPServer: apiconfig.PublicHTTPServer{
			WriteTimeout: 15 * time.Second,
		},
		Auth: apiconfig.Auth{
			Enabled:   false,
			SkipPaths: []string{"/v1/health", "/v1/liveness", "/v1/readiness"},
		},
		Telemetry: apiconfig.Telemetry{
			Metrics: apiconfig.Metrics{Enabled: false},
			Traces:  apiconfig.Traces{Enabled: false},
		},
		Logging: apiconfig.Logging{
			AccessLog: apiconfig.AccessLog{
				Enabled:         false,
				LogHealthChecks: false,
			},
		},
	}

	router := internalhttp.NewRouter(internalhttp.RouterConfig{
		App:           apiApp,
		Logger:        log,
		MetricsClient: metricsClient,
		ServiceConfig: cfg,
	})

	httpServer := httptest.NewServer(router)

	return &IntegrationTestServer{
		HTTPServer: httpServer,
		TestServer: testServer,
		GRPCClient: grpcClient,
		grpcConn:   conn,
	}, nil
}

// Close shuts down all test resources.
func (s *IntegrationTestServer) Close() {
	if s.HTTPServer != nil {
		s.HTTPServer.Close()
	}

	if s.GRPCClient != nil {
		s.GRPCClient.Close()
	}

	if s.grpcConn != nil {
		s.grpcConn.Close()
	}

	if s.TestServer != nil {
		s.TestServer.Close()
	}
}

// TruncateDevices removes all devices from the database.
func (s *IntegrationTestServer) TruncateDevices(ctx context.Context) error {
	return s.TestServer.TruncateDevices(ctx)
}

// URL returns the base URL of the HTTP server.
func (s *IntegrationTestServer) URL() string {
	return s.HTTPServer.URL
}

// Client returns the HTTP client for making requests.
func (s *IntegrationTestServer) Client() *http.Client {
	return s.HTTPServer.Client()
}

// RequestOption configures an HTTP request.
type RequestOption func(*http.Request)

// WithBody sets the request body and content type.
func WithBody(body io.Reader) RequestOption {
	return func(req *http.Request) {
		if body != nil {
			req.Body = io.NopCloser(body)
			req.Header.Set("Content-Type", "application/json")
		}
	}
}

// WithIdempotencyKey sets a unique idempotency key.
func WithIdempotencyKey() RequestOption {
	return func(req *http.Request) {
		req.Header.Set("Idempotency-Key", model.NewDeviceID().String())
	}
}

// DoRequest performs an HTTP request with the given method, path, and options.
func (s *IntegrationTestServer) DoRequest(ctx context.Context, method, path string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.URL()+path, nil)
	if err != nil {
		return nil, err
	}

	if method != http.MethodOptions {
		SetAuthHeader(req)
	}

	for _, opt := range opts {
		opt(req)
	}

	return s.Client().Do(req)
}

// Get performs a GET request.
func (s *IntegrationTestServer) Get(ctx context.Context, path string) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodGet, path)
}

// Head performs a HEAD request.
func (s *IntegrationTestServer) Head(ctx context.Context, path string) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodHead, path)
}

// Options performs an OPTIONS request.
func (s *IntegrationTestServer) Options(ctx context.Context, path string) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodOptions, path)
}

// Post performs a POST request with a body and idempotency key.
func (s *IntegrationTestServer) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodPost, path, WithBody(body), WithIdempotencyKey())
}

// Put performs a PUT request with a body.
func (s *IntegrationTestServer) Put(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodPut, path, WithBody(body))
}

// Patch performs a PATCH request with a body.
func (s *IntegrationTestServer) Patch(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodPatch, path, WithBody(body))
}

// Delete performs a DELETE request.
func (s *IntegrationTestServer) Delete(ctx context.Context, path string) (*http.Response, error) {
	return s.DoRequest(ctx, http.MethodDelete, path)
}

// TestAuthToken is a test token for authentication.
const TestAuthToken = "v4.public.test-token-for-integration-tests"

// SetAuthHeader sets the Authorization header with the test token.
func SetAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+TestAuthToken)
}

// SetIdempotencyKey sets a unique idempotency key header.
func SetIdempotencyKey(req *http.Request) {
	req.Header.Set("Idempotency-Key", model.NewDeviceID().String())
}

// DecodeJSON decodes a JSON response body.
func DecodeJSON(body io.Reader, dest any) error {
	return json.NewDecoder(body).Decode(dest)
}

func getMigrationsPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	// From itest/testhelpers.go, go up to svc-api-gateway, then to svc-devices/migrations
	serviceRoot := filepath.Dir(filepath.Dir(currentFile))
	migrationsPath := filepath.Join(serviceRoot, "..", "svc-devices", "migrations")

	return filepath.Abs(migrationsPath)
}
