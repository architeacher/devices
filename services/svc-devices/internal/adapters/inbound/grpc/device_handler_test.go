package grpc_test

import (
	"context"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-devices/internal/mocks"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func createTestApp(svc *mocks.FakeDevicesService, dbChecker *mocks.FakeDatabaseHealthChecker) *usecases.Application {
	log := logger.New("debug", "console")
	tp := infrastructure.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	return usecases.NewApplication(svc, dbChecker, log, tp, mc)
}

func TestDeviceHandler_CreateDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mocks.FakeDevicesService)
		request      *devicev1.CreateDeviceRequest
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully create device",
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.CreateDeviceStub = func(_ context.Context, name, brand string, state model.State) (*model.Device, error) {
					return model.NewDevice(name, brand, state), nil
				}
			},
			request: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "missing name returns invalid argument",
			setupSvc: func(_ *mocks.FakeDevicesService) {
			},
			request: &devicev1.CreateDeviceRequest{
				Name:  "",
				Brand: "Test Brand",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
		{
			name: "missing brand returns invalid argument",
			setupSvc: func(_ *mocks.FakeDevicesService) {
			},
			request: &devicev1.CreateDeviceRequest{
				Name:  "Test Device",
				Brand: "",
				State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			tc.setupSvc(svc)
			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			dbChecker.PingReturns(nil)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.CreateDevice(t.Context(), tc.request)

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Device)
				require.NotEmpty(t, resp.Device.Id)
				require.Equal(t, tc.request.Name, resp.Device.Name)
				require.Equal(t, tc.request.Brand, resp.Device.Brand)
			}
		})
	}
}

func TestDeviceHandler_GetDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mocks.FakeDevicesService) string
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully get device",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				fake.GetDeviceReturns(device, nil)

				return device.ID.String()
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "device not found",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				fake.GetDeviceReturns(nil, model.ErrDeviceNotFound)

				return model.NewDeviceID().String()
			},
			expectedCode: codes.NotFound,
			expectError:  true,
		},
		{
			name: "invalid device ID",
			setupSvc: func(_ *mocks.FakeDevicesService) string {
				return "invalid-uuid"
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
		{
			name: "empty device ID",
			setupSvc: func(_ *mocks.FakeDevicesService) string {
				return ""
			},
			expectedCode: codes.InvalidArgument,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			dbChecker.PingReturns(nil)
			deviceID := tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.GetDevice(t.Context(), &devicev1.GetDeviceRequest{Id: deviceID})

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Device)
				require.Equal(t, deviceID, resp.Device.Id)
			}
		})
	}
}

func TestDeviceHandler_ListDevices(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		setupSvc      func(*mocks.FakeDevicesService)
		request       *devicev1.ListDevicesRequest
		expectedCount int
	}{
		{
			name: "list all devices",
			setupSvc: func(fake *mocks.FakeDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				device2 := model.NewDevice("Device 2", "Brand B", model.StateInUse)
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{device1, device2},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 2,
						TotalPages: 1,
					},
					Filters: model.DefaultDeviceFilter(),
				}, nil)
			},
			request:       &devicev1.ListDevicesRequest{},
			expectedCount: 2,
		},
		{
			name: "filter by brand",
			setupSvc: func(fake *mocks.FakeDevicesService) {
				device1 := model.NewDevice("Device 1", "Brand A", model.StateAvailable)
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{device1},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 1,
						TotalPages: 1,
					},
					Filters: model.DeviceFilter{Brands: []string{"Brand A"}, Page: 1, Size: 10},
				}, nil)
			},
			request: &devicev1.ListDevicesRequest{
				Brands: []string{"Brand A"},
			},
			expectedCount: 1,
		},
		{
			name: "empty list",
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.ListDevicesReturns(&model.DeviceList{
					Devices: []*model.Device{},
					Pagination: model.Pagination{
						Page:       1,
						Size:       10,
						TotalItems: 0,
						TotalPages: 1,
					},
					Filters: model.DefaultDeviceFilter(),
				}, nil)
			},
			request:       &devicev1.ListDevicesRequest{},
			expectedCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			dbChecker.PingReturns(nil)
			tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.ListDevices(t.Context(), tc.request)

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Len(t, resp.Devices, tc.expectedCount)
		})
	}
}

func TestDeviceHandler_UpdateDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mocks.FakeDevicesService) string
		request      func(id string) *devicev1.UpdateDeviceRequest
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully update device",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				device := model.NewDevice("Original", "Original Brand", model.StateAvailable)
				fake.UpdateDeviceStub = func(_ context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
					return &model.Device{
						ID:    id,
						Name:  name,
						Brand: brand,
						State: state,
					}, nil
				}

				return device.ID.String()
			},
			request: func(id string) *devicev1.UpdateDeviceRequest {
				return &devicev1.UpdateDeviceRequest{
					Id:    id,
					Name:  "Updated Name",
					Brand: "Updated Brand",
					State: devicev1.DeviceState_DEVICE_STATE_IN_USE,
				}
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "cannot update name of in-use device",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				device := model.NewDevice("Original", "Original Brand", model.StateInUse)
				fake.UpdateDeviceReturns(nil, model.ErrCannotUpdateInUseDevice)

				return device.ID.String()
			},
			request: func(id string) *devicev1.UpdateDeviceRequest {
				return &devicev1.UpdateDeviceRequest{
					Id:    id,
					Name:  "New Name",
					Brand: "Original Brand",
					State: devicev1.DeviceState_DEVICE_STATE_IN_USE,
				}
			},
			expectedCode: codes.FailedPrecondition,
			expectError:  true,
		},
		{
			name: "device not found",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				fake.UpdateDeviceReturns(nil, model.ErrDeviceNotFound)

				return model.NewDeviceID().String()
			},
			request: func(id string) *devicev1.UpdateDeviceRequest {
				return &devicev1.UpdateDeviceRequest{
					Id:    id,
					Name:  "Name",
					Brand: "Brand",
					State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
				}
			},
			expectedCode: codes.NotFound,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			dbChecker.PingReturns(nil)
			deviceID := tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.UpdateDevice(t.Context(), tc.request(deviceID))

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Device)
			}
		})
	}
}

func TestDeviceHandler_DeleteDevice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setupSvc     func(*mocks.FakeDevicesService) string
		expectedCode codes.Code
		expectError  bool
	}{
		{
			name: "successfully delete device",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				fake.DeleteDeviceReturns(nil)

				return device.ID.String()
			},
			expectedCode: codes.OK,
			expectError:  false,
		},
		{
			name: "cannot delete in-use device",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				device := model.NewDevice("Test", "Brand", model.StateInUse)
				fake.DeleteDeviceReturns(model.ErrCannotDeleteInUseDevice)

				return device.ID.String()
			},
			expectedCode: codes.FailedPrecondition,
			expectError:  true,
		},
		{
			name: "device not found",
			setupSvc: func(fake *mocks.FakeDevicesService) string {
				fake.DeleteDeviceReturns(model.ErrDeviceNotFound)

				return model.NewDeviceID().String()
			},
			expectedCode: codes.NotFound,
			expectError:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			dbChecker.PingReturns(nil)
			deviceID := tc.setupSvc(svc)
			app := createTestApp(svc, dbChecker)
			handler := inboundgrpc.NewDevicesHandler(app)

			resp, err := handler.DeleteDevice(t.Context(), &devicev1.DeleteDeviceRequest{Id: deviceID})

			if tc.expectError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, tc.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
