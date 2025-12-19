package itest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	internalhttp "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

type (
	MockDeviceService struct {
		mu      sync.RWMutex
		devices map[string]*model.Device

		CreateDeviceFn func(ctx context.Context, name, brand string, state model.State) (*model.Device, error)
		GetDeviceFn    func(ctx context.Context, id model.DeviceID) (*model.Device, error)
		ListDevicesFn  func(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
		UpdateDeviceFn func(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error)
		PatchDeviceFn  func(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error)
		DeleteDeviceFn func(ctx context.Context, id model.DeviceID) error
	}

	MockHealthChecker struct {
		LivenessFn  func(ctx context.Context) (*model.LivenessReport, error)
		ReadinessFn func(ctx context.Context) (*model.ReadinessReport, error)
		HealthFn    func(ctx context.Context) (*model.HealthReport, error)
	}

	TestServer struct {
		Server        *httptest.Server
		DeviceService *MockDeviceService
		HealthChecker *MockHealthChecker
	}
)

func NewMockDeviceService() *MockDeviceService {
	return &MockDeviceService{
		devices: make(map[string]*model.Device),
	}
}

func (m *MockDeviceService) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	if m.CreateDeviceFn != nil {
		return m.CreateDeviceFn(ctx, name, brand, state)
	}

	device := model.NewDevice(name, brand, state)
	m.mu.Lock()
	m.devices[device.ID.String()] = device
	m.mu.Unlock()

	return device, nil
}

func (m *MockDeviceService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	if m.GetDeviceFn != nil {
		return m.GetDeviceFn(ctx, id)
	}

	m.mu.RLock()
	device, ok := m.devices[id.String()]
	m.mu.RUnlock()

	if !ok {
		return nil, model.ErrDeviceNotFound
	}

	return device, nil
}

func (m *MockDeviceService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	if m.ListDevicesFn != nil {
		return m.ListDevicesFn(ctx, filter)
	}

	m.mu.RLock()
	devices := make([]*model.Device, 0, len(m.devices))
	for _, device := range m.devices {
		if filter.Brand != nil && device.Brand != *filter.Brand {
			continue
		}
		if filter.State != nil && device.State != *filter.State {
			continue
		}
		devices = append(devices, device)
	}
	m.mu.RUnlock()

	totalItems := uint(len(devices))
	totalPages := uint(1)
	if filter.Size > 0 && totalItems > 0 {
		totalPages = (totalItems + filter.Size - 1) / filter.Size
	}

	start := (filter.Page - 1) * filter.Size
	end := start + filter.Size
	if start > totalItems {
		start = totalItems
	}
	if end > totalItems {
		end = totalItems
	}

	return &model.DeviceList{
		Devices: devices[start:end],
		Pagination: model.Pagination{
			Page:        filter.Page,
			Size:        filter.Size,
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			HasNext:     filter.Page < totalPages,
			HasPrevious: filter.Page > 1,
		},
		Filters: filter,
	}, nil
}

func (m *MockDeviceService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	if m.UpdateDeviceFn != nil {
		return m.UpdateDeviceFn(ctx, id, name, brand, state)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id.String()]
	if !ok {
		return nil, model.ErrDeviceNotFound
	}

	if err := device.Update(name, brand, state); err != nil {
		return nil, err
	}

	return device, nil
}

func (m *MockDeviceService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	if m.PatchDeviceFn != nil {
		return m.PatchDeviceFn(ctx, id, updates)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id.String()]
	if !ok {
		return nil, model.ErrDeviceNotFound
	}

	name := device.Name
	brand := device.Brand
	state := device.State

	if v, exists := updates["name"]; exists {
		name = v.(string)
	}
	if v, exists := updates["brand"]; exists {
		brand = v.(string)
	}
	if v, exists := updates["state"]; exists {
		state = model.State(v.(string))
	}

	if err := device.Update(name, brand, state); err != nil {
		return nil, err
	}

	return device, nil
}

func (m *MockDeviceService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	if m.DeleteDeviceFn != nil {
		return m.DeleteDeviceFn(ctx, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id.String()]
	if !ok {
		return model.ErrDeviceNotFound
	}

	if !device.CanDelete() {
		return model.ErrCannotDeleteInUseDevice
	}

	delete(m.devices, id.String())

	return nil
}

func (m *MockDeviceService) AddDevice(device *model.Device) {
	m.mu.Lock()
	m.devices[device.ID.String()] = device
	m.mu.Unlock()
}

func (m *MockDeviceService) Clear() {
	m.mu.Lock()
	m.devices = make(map[string]*model.Device)
	m.mu.Unlock()
}

func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{}
}

func (m *MockHealthChecker) Liveness(ctx context.Context) (*model.LivenessReport, error) {
	if m.LivenessFn != nil {
		return m.LivenessFn(ctx)
	}

	return &model.LivenessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}, nil
}

func (m *MockHealthChecker) Readiness(ctx context.Context) (*model.ReadinessReport, error) {
	if m.ReadinessFn != nil {
		return m.ReadinessFn(ctx)
	}

	return &model.ReadinessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Checks: map[string]model.DependencyCheck{
			"storage": {
				Status:      model.DependencyStatusUp,
				LatencyMs:   1,
				Message:     "ok",
				LastChecked: time.Now().UTC(),
			},
		},
	}, nil
}

func (m *MockHealthChecker) Health(ctx context.Context) (*model.HealthReport, error) {
	if m.HealthFn != nil {
		return m.HealthFn(ctx)
	}

	return &model.HealthReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version: model.VersionInfo{
			API:   "1.0.0",
			Build: "test",
			Go:    "1.25",
		},
		Checks: map[string]model.DependencyCheck{
			"storage": {
				Status:      model.DependencyStatusUp,
				LatencyMs:   1,
				Message:     "ok",
				LastChecked: time.Now().UTC(),
			},
		},
		System: model.SystemInfo{
			Goroutines: 10,
			CPUCores:   4,
		},
	}, nil
}

func NewTestServer() *TestServer {
	return NewTestServerWithAuth(false)
}

func NewTestServerWithAuth(authEnabled bool) *TestServer {
	deviceService := NewMockDeviceService()
	healthChecker := NewMockHealthChecker()

	log := logger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()
	tracerProvider := otelNoop.NewTracerProvider()

	app := usecases.NewWebApplication(deviceService, healthChecker, log, tracerProvider, metricsClient)

	cfg := &config.ServiceConfig{
		App: config.AppConfig{
			ServiceName:    "api-gateway-test",
			ServiceVersion: "1.0.0",
			Env:            "test",
		},
		HTTPServer: config.HTTPServerConfig{
			WriteTimeout: 15 * time.Second,
		},
		Auth: config.AuthConfig{
			Enabled:   authEnabled,
			SkipPaths: []string{"/v1/health", "/v1/liveness", "/v1/readiness"},
		},
		Telemetry: config.TelemetryConfig{
			Metrics: config.Metrics{Enabled: false},
			Traces:  config.Traces{Enabled: false},
		},
		Logging: config.LoggingConfig{
			AccessLog: config.AccessLogConfig{
				Enabled:         false,
				LogHealthChecks: false,
			},
		},
	}

	router := internalhttp.NewRouter(internalhttp.RouterConfig{
		App:           app,
		Logger:        log,
		MetricsClient: metricsClient,
		Config:        cfg,
	})

	server := httptest.NewServer(router)

	return &TestServer{
		Server:        server,
		DeviceService: deviceService,
		HealthChecker: healthChecker,
	}
}

func (ts *TestServer) Close() {
	ts.Server.Close()
}

func (ts *TestServer) URL() string {
	return ts.Server.URL
}

func (ts *TestServer) Client() *http.Client {
	return ts.Server.Client()
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

// WithIdempotencyKey sets the Idempotency-Key header.
func WithIdempotencyKey() RequestOption {
	return func(req *http.Request) {
		req.Header.Set("Idempotency-Key", model.NewDeviceID().String())
	}
}

// DoRequest performs an HTTP request with the given method, path, and options.
func (ts *TestServer) DoRequest(ctx context.Context, method, path string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, ts.URL()+path, nil)
	if err != nil {
		return nil, err
	}

	if method != http.MethodOptions {
		SetAuthHeader(req)
	}

	for _, opt := range opts {
		opt(req)
	}

	return ts.Client().Do(req)
}

// Get performs a GET request with authentication.
func (ts *TestServer) Get(ctx context.Context, path string) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodGet, path)
}

// Head performs a HEAD request with authentication.
func (ts *TestServer) Head(ctx context.Context, path string) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodHead, path)
}

// Options performs an OPTIONS request (no auth needed for preflight).
func (ts *TestServer) Options(ctx context.Context, path string) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodOptions, path)
}

// Post performs a POST request with authentication and idempotency key.
func (ts *TestServer) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodPost, path, WithBody(body), WithIdempotencyKey())
}

// Put performs a PUT request with authentication.
func (ts *TestServer) Put(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodPut, path, WithBody(body))
}

// Patch performs a PATCH request with authentication.
func (ts *TestServer) Patch(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodPatch, path, WithBody(body))
}

// Delete performs a DELETE request with authentication.
func (ts *TestServer) Delete(ctx context.Context, path string) (*http.Response, error) {
	return ts.DoRequest(ctx, http.MethodDelete, path)
}

// TestAuthToken is a dummy PASETO v4 token for testing when auth is disabled.
const TestAuthToken = "v4.public.test-token-for-integration-tests"

// SetAuthHeader sets the Authorization header with a test token.
func SetAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+TestAuthToken)
}

// SetIdempotencyKey sets the Idempotency-Key header for POST requests.
func SetIdempotencyKey(req *http.Request) {
	req.Header.Set("Idempotency-Key", model.NewDeviceID().String())
}

// DecodeJSON decodes a JSON response body into the provided destination.
func DecodeJSON(body io.Reader, dest any) error {
	return json.NewDecoder(body).Decode(dest)
}
