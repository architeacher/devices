package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/circuitbreaker"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testConfig() *config.ServiceConfig {
	return &config.ServiceConfig{
		DevicesGRPCClient: config.DevicesGRPCClient{
			Address:        "localhost:9090",
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			MaxMessageSize: 4194304,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled: false,
			},
			TLS: config.TLSConfig{
				Enabled: false,
			},
		},
		Backoff: config.Backoff{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   10 * time.Second,
		},
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *config.ServiceConfig
	}{
		{
			name: "creates client with valid config",
			cfg:  testConfig(),
		},
		{
			name: "creates client with circuit breaker enabled",
			cfg: func() *config.ServiceConfig {
				cfg := testConfig()
				cfg.DevicesGRPCClient.CircuitBreaker.Enabled = true
				cfg.DevicesGRPCClient.CircuitBreaker.MaxRequests = 5
				cfg.DevicesGRPCClient.CircuitBreaker.Interval = 60 * time.Second
				cfg.DevicesGRPCClient.CircuitBreaker.Timeout = 30 * time.Second
				cfg.DevicesGRPCClient.CircuitBreaker.FailureThreshold = 5

				return cfg
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockDevice := &mocks.FakeDeviceServiceClient{}
			mockHealth := &mocks.FakeHealthServiceClient{}

			client := NewClient(nil, tc.cfg,
				WithDeviceClient(mockDevice),
				WithHealthClient(mockHealth),
			)

			require.NotNil(t, client)
			require.NotNil(t, client.Config())
		})
	}
}

func TestNewClient_DefaultsAreSetAfterOptions(t *testing.T) {
	t.Parallel()

	mockDevice := &mocks.FakeDeviceServiceClient{}

	cfg := testConfig()
	cfg.DevicesGRPCClient.CircuitBreaker.Enabled = true
	cfg.DevicesGRPCClient.CircuitBreaker.FailureThreshold = 5

	client := NewClient(nil, cfg, WithDeviceClient(mockDevice))

	require.NotNil(t, client)
	require.NotNil(t, client.cb)
}

func TestClient_CreateDevice(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	cases := []struct {
		name    string
		setup   func() *mocks.FakeDeviceServiceClient
		cb      *circuitbreaker.CircuitBreaker[any]
		req     *devicev1.CreateDeviceRequest
		wantID  string
		wantErr bool
		errIs   error
	}{
		{
			name: "makes gRPC call and returns response",
			setup: func() *mocks.FakeDeviceServiceClient {
				mock := &mocks.FakeDeviceServiceClient{}
				mock.CreateDeviceStub = func(_ context.Context, in *devicev1.CreateDeviceRequest, _ ...grpc.CallOption) (*devicev1.CreateDeviceResponse, error) {
					return &devicev1.CreateDeviceResponse{
						Device: &devicev1.Device{
							Id:        "123e4567-e89b-12d3-a456-426614174000",
							Name:      in.Name,
							Brand:     in.Brand,
							State:     in.State,
							CreatedAt: timestamppb.New(now),
							UpdatedAt: timestamppb.New(now),
						},
					}, nil
				}

				return mock
			},
			req: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			wantID:  "123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name: "returns raw gRPC error",
			setup: func() *mocks.FakeDeviceServiceClient {
				mock := &mocks.FakeDeviceServiceClient{}
				mock.CreateDeviceReturns(nil, errors.New("connection refused"))

				return mock
			},
			req: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			wantErr: true,
		},
		{
			name: "returns circuit breaker error when open",
			setup: func() *mocks.FakeDeviceServiceClient {
				mock := &mocks.FakeDeviceServiceClient{}
				mock.CreateDeviceReturns(nil, errors.New("should not be called"))

				return mock
			},
			cb: func() *circuitbreaker.CircuitBreaker[any] {
				cb := circuitbreaker.New[any](circuitbreaker.Config{
					Name:             "test-circuit-breaker",
					Enabled:          true,
					MaxRequests:      1,
					Interval:         1 * time.Second,
					Timeout:          1 * time.Second,
					FailureThreshold: 1,
				})
				_, _ = circuitbreaker.Execute(cb, func() (any, error) {
					return nil, errors.New("trip")
				})

				return cb
			}(),
			req: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			wantErr: true,
			errIs:   circuitbreaker.ErrCircuitOpen,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := tc.setup()
			client := NewClient(nil, testConfig(),
				WithDeviceClient(mock),
				WithCircuitBreaker(tc.cb),
			)

			resp, err := client.CreateDevice(t.Context(), tc.req)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tc.wantID, resp.GetDevice().GetId())
		})
	}
}

func TestClient_DeleteDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		setup   func() *mocks.FakeDeviceServiceClient
		req     *devicev1.DeleteDeviceRequest
		wantErr bool
	}{
		{
			name: "makes gRPC delete call",
			setup: func() *mocks.FakeDeviceServiceClient {
				mock := &mocks.FakeDeviceServiceClient{}
				mock.DeleteDeviceReturns(&emptypb.Empty{}, nil)

				return mock
			},
			req: &devicev1.DeleteDeviceRequest{
				Id: "123e4567-e89b-12d3-a456-426614174000",
			},
			wantErr: false,
		},
		{
			name: "returns raw gRPC error",
			setup: func() *mocks.FakeDeviceServiceClient {
				mock := &mocks.FakeDeviceServiceClient{}
				mock.DeleteDeviceReturns(nil, errors.New("not found"))

				return mock
			},
			req: &devicev1.DeleteDeviceRequest{
				Id: "123e4567-e89b-12d3-a456-426614174000",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := tc.setup()
			client := NewClient(nil, testConfig(), WithDeviceClient(mock))

			_, err := client.DeleteDevice(t.Context(), tc.req)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestClient_CheckHealth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		setup      func() *mocks.FakeHealthServiceClient
		req        *devicev1.HealthCheckRequest
		wantStatus devicev1.HealthCheckResponse_ServingStatus
		wantErr    bool
	}{
		{
			name: "makes gRPC call and returns response",
			setup: func() *mocks.FakeHealthServiceClient {
				mock := &mocks.FakeHealthServiceClient{}
				mock.CheckReturns(&devicev1.HealthCheckResponse{
					Status: devicev1.HealthCheckResponse_SERVING_STATUS_SERVING,
				}, nil)

				return mock
			},
			req:        &devicev1.HealthCheckRequest{},
			wantStatus: devicev1.HealthCheckResponse_SERVING_STATUS_SERVING,
			wantErr:    false,
		},
		{
			name: "returns not serving status",
			setup: func() *mocks.FakeHealthServiceClient {
				mock := &mocks.FakeHealthServiceClient{}
				mock.CheckReturns(&devicev1.HealthCheckResponse{
					Status: devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING,
				}, nil)

				return mock
			},
			req:        &devicev1.HealthCheckRequest{},
			wantStatus: devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING,
			wantErr:    false,
		},
		{
			name: "returns raw gRPC error",
			setup: func() *mocks.FakeHealthServiceClient {
				mock := &mocks.FakeHealthServiceClient{}
				mock.CheckReturns(nil, errors.New("connection refused"))

				return mock
			},
			req:     &devicev1.HealthCheckRequest{},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := tc.setup()
			client := NewClient(nil, testConfig(), WithHealthClient(mock))

			resp, err := client.CheckHealth(t.Context(), tc.req)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tc.wantStatus, resp.GetStatus())
		})
	}
}
