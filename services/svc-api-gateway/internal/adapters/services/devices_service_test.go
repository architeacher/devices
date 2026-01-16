package services

import (
	"context"
	"errors"
	"testing"
	"time"

	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	grpcclient "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/outbound/grpc"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- Test Helpers ---

func testConfig() *config.ServiceConfig {
	return &config.ServiceConfig{
		App: config.App{
			APIVersion: "v1",
		},
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

// --- Service Tests ---

func TestNewDevicesService(t *testing.T) {
	t.Parallel()

	fakeDevice := &mocks.FakeDeviceServiceClient{}
	fakeHealth := &mocks.FakeHealthServiceClient{}

	client := grpcclient.NewClient(nil, testConfig(),
		grpcclient.WithDeviceClient(fakeDevice),
		grpcclient.WithHealthClient(fakeHealth),
	)

	svc := NewDevicesService(client)

	require.NotNil(t, svc)
}

func TestDevicesService_CreateDevice(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	deviceID := "123e4567-e89b-12d3-a456-426614174000"

	cases := []struct {
		name      string
		setupMock func(*mocks.FakeDeviceServiceClient)
		device    struct{ name, brand string; state model.State }
		wantID    string
		wantErr   bool
		errIs     error
	}{
		{
			name: "creates device and maps domain correctly",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.CreateDeviceStub = func(_ context.Context, in *devicev1.CreateDeviceRequest, _ ...grpc.CallOption) (*devicev1.CreateDeviceResponse, error) {
					return &devicev1.CreateDeviceResponse{
						Device: &devicev1.Device{
							Id:        deviceID,
							Name:      in.Name,
							Brand:     in.Brand,
							State:     in.State,
							CreatedAt: timestamppb.New(now),
							UpdatedAt: timestamppb.New(now),
						},
					}, nil
				}
			},
			device:  struct{ name, brand string; state model.State }{"Test Device", "Test Brand", model.StateAvailable},
			wantID:  deviceID,
			wantErr: false,
		},
		{
			name: "maps gRPC NotFound error to domain error",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.CreateDeviceReturns(nil, status.Error(codes.NotFound, "device not found"))
			},
			device:  struct{ name, brand string; state model.State }{"Test Device", "Test Brand", model.StateAvailable},
			wantErr: true,
			errIs:   model.ErrDeviceNotFound,
		},
		{
			name: "maps gRPC Unavailable error to domain error",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.CreateDeviceReturns(nil, status.Error(codes.Unavailable, "service unavailable"))
			},
			device:  struct{ name, brand string; state model.State }{"Test Device", "Test Brand", model.StateAvailable},
			wantErr: true,
			errIs:   model.ErrServiceUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &mocks.FakeDeviceServiceClient{}
			tc.setupMock(fake)

			client := grpcclient.NewClient(nil, testConfig(),
				grpcclient.WithDeviceClient(fake),
			)
			svc := NewDevicesService(client)

			device, err := svc.CreateDevice(t.Context(), tc.device.name, tc.device.brand, tc.device.state)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, device)
			require.Equal(t, tc.wantID, device.ID.String())
		})
	}
}

func TestDevicesService_GetDevice(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	deviceID, _ := model.ParseDeviceID("123e4567-e89b-12d3-a456-426614174000")

	cases := []struct {
		name      string
		setupMock func(*mocks.FakeDeviceServiceClient)
		deviceID  model.DeviceID
		wantName  string
		wantErr   bool
		errIs     error
	}{
		{
			name: "gets device and maps domain correctly",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.GetDeviceStub = func(_ context.Context, in *devicev1.GetDeviceRequest, _ ...grpc.CallOption) (*devicev1.GetDeviceResponse, error) {
					return &devicev1.GetDeviceResponse{
						Device: &devicev1.Device{
							Id:        in.Id,
							Name:      "Test Device",
							Brand:     "Test Brand",
							State:     devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
							CreatedAt: timestamppb.New(now),
							UpdatedAt: timestamppb.New(now),
						},
					}, nil
				}
			},
			deviceID: deviceID,
			wantName: "Test Device",
			wantErr:  false,
		},
		{
			name: "maps gRPC NotFound error to domain error",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.GetDeviceReturns(nil, status.Error(codes.NotFound, "device not found"))
			},
			deviceID: deviceID,
			wantErr:  true,
			errIs:    model.ErrDeviceNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &mocks.FakeDeviceServiceClient{}
			tc.setupMock(fake)

			client := grpcclient.NewClient(nil, testConfig(),
				grpcclient.WithDeviceClient(fake),
			)
			svc := NewDevicesService(client)

			device, err := svc.GetDevice(t.Context(), tc.deviceID)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, device)
			require.Equal(t, tc.wantName, device.Name)
		})
	}
}

func TestDevicesService_DeleteDevice(t *testing.T) {
	t.Parallel()

	deviceID, _ := model.ParseDeviceID("123e4567-e89b-12d3-a456-426614174000")

	cases := []struct {
		name      string
		setupMock func(*mocks.FakeDeviceServiceClient)
		deviceID  model.DeviceID
		wantErr   bool
		errIs     error
	}{
		{
			name: "deletes device successfully",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.DeleteDeviceReturns(&emptypb.Empty{}, nil)
			},
			deviceID: deviceID,
			wantErr:  false,
		},
		{
			name: "maps gRPC FailedPrecondition for in-use device",
			setupMock: func(fake *mocks.FakeDeviceServiceClient) {
				fake.DeleteDeviceReturns(nil, status.Error(codes.FailedPrecondition, "cannot delete device in use"))
			},
			deviceID: deviceID,
			wantErr:  true,
			errIs:    model.ErrCannotDeleteInUseDevice,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &mocks.FakeDeviceServiceClient{}
			tc.setupMock(fake)

			client := grpcclient.NewClient(nil, testConfig(),
				grpcclient.WithDeviceClient(fake),
			)
			svc := NewDevicesService(client)

			err := svc.DeleteDevice(t.Context(), tc.deviceID)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestDevicesService_Health(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		setupMock  func(*mocks.FakeHealthServiceClient)
		wantStatus model.HealthStatus
	}{
		{
			name: "returns healthy status when serving",
			setupMock: func(fake *mocks.FakeHealthServiceClient) {
				fake.CheckReturns(&devicev1.HealthCheckResponse{
					Status: devicev1.HealthCheckResponse_SERVING_STATUS_SERVING,
				}, nil)
			},
			wantStatus: model.HealthStatusOK,
		},
		{
			name: "returns down status when not serving",
			setupMock: func(fake *mocks.FakeHealthServiceClient) {
				fake.CheckReturns(&devicev1.HealthCheckResponse{
					Status: devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING,
				}, nil)
			},
			wantStatus: model.HealthStatusDown,
		},
		{
			name: "returns down status on error",
			setupMock: func(fake *mocks.FakeHealthServiceClient) {
				fake.CheckReturns(nil, errors.New("connection refused"))
			},
			wantStatus: model.HealthStatusDown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &mocks.FakeHealthServiceClient{}
			tc.setupMock(fake)

			client := grpcclient.NewClient(nil, testConfig(),
				grpcclient.WithHealthClient(fake),
			)
			svc := NewDevicesService(client)

			report, err := svc.Health(t.Context())

			require.NoError(t, err)
			require.NotNil(t, report)
			require.Equal(t, tc.wantStatus, report.Status)
		})
	}
}
