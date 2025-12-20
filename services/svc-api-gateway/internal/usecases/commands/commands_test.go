package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/commands"
	"github.com/stretchr/testify/require"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

func TestCreateDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	tp := otelNoop.NewTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		cmd         commands.CreateDeviceCommand
		setupSvc    func(*mocks.FakeDevicesService)
		expectError bool
	}{
		{
			name: "create available device",
			cmd: commands.CreateDeviceCommand{
				Name:  "iPhone",
				Brand: "Apple",
				State: model.StateAvailable,
			},
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.CreateDeviceStub = func(_ context.Context, name, brand string, state model.State) (*model.Device, error) {
					return model.NewDevice(name, brand, state), nil
				}
			},
			expectError: false,
		},
		{
			name: "create in-use device",
			cmd: commands.CreateDeviceCommand{
				Name:  "Pixel",
				Brand: "Google",
				State: model.StateInUse,
			},
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.CreateDeviceStub = func(_ context.Context, name, brand string, state model.State) (*model.Device, error) {
					return model.NewDevice(name, brand, state), nil
				}
			},
			expectError: false,
		},
		{
			name: "service error",
			cmd: commands.CreateDeviceCommand{
				Name:  "Test",
				Brand: "Brand",
				State: model.StateAvailable,
			},
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.CreateDeviceReturns(nil, errors.New("service error"))
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			if tc.setupSvc != nil {
				tc.setupSvc(svc)
			}

			handler := commands.NewCreateDeviceCommandHandler(svc, log, tp, mc)
			device, err := handler.Handle(t.Context(), tc.cmd)

			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, device)
			} else {
				require.NoError(t, err)
				require.NotNil(t, device)
				require.Equal(t, tc.cmd.Name, device.Name)
				require.Equal(t, tc.cmd.Brand, device.Brand)
				require.Equal(t, tc.cmd.State, device.State)
			}
		})
	}
}

func TestUpdateDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	tp := otelNoop.NewTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mocks.FakeDevicesService) model.DeviceID
		newName     string
		newBrand    string
		newState    model.State
		expectError bool
	}{
		{
			name: "successfully update device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				id := model.NewDeviceID()
				fake.UpdateDeviceStub = func(_ context.Context, deviceID model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
					return &model.Device{
						ID:    deviceID,
						Name:  name,
						Brand: brand,
						State: state,
					}, nil
				}

				return id
			},
			newName:     "Updated",
			newBrand:    "Updated Brand",
			newState:    model.StateInUse,
			expectError: false,
		},
		{
			name: "device not found",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				fake.UpdateDeviceReturns(nil, model.ErrDeviceNotFound)

				return model.NewDeviceID()
			},
			newName:     "Name",
			newBrand:    "Brand",
			newState:    model.StateAvailable,
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			deviceID := tc.setupSvc(svc)

			handler := commands.NewUpdateDeviceCommandHandler(svc, log, tp, mc)

			cmd := commands.UpdateDeviceCommand{
				ID:    deviceID,
				Name:  tc.newName,
				Brand: tc.newBrand,
				State: tc.newState,
			}

			device, err := handler.Handle(t.Context(), cmd)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, device)
				require.Equal(t, deviceID, device.ID)
				require.Equal(t, tc.newName, device.Name)
				require.Equal(t, tc.newBrand, device.Brand)
				require.Equal(t, tc.newState, device.State)
			}
		})
	}
}

func TestPatchDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	tp := otelNoop.NewTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mocks.FakeDevicesService) model.DeviceID
		updates     map[string]any
		expectError bool
		validate    func(*testing.T, *model.Device)
	}{
		{
			name: "patch name of device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				id := model.NewDeviceID()
				fake.PatchDeviceStub = func(_ context.Context, deviceID model.DeviceID, _ map[string]any) (*model.Device, error) {
					return &model.Device{
						ID:    deviceID,
						Name:  "Patched Name",
						Brand: "Brand",
						State: model.StateAvailable,
					}, nil
				}

				return id
			},
			updates: map[string]any{
				"name": "Patched Name",
			},
			expectError: false,
			validate: func(t *testing.T, d *model.Device) {
				require.Equal(t, "Patched Name", d.Name)
				require.Equal(t, "Brand", d.Brand)
			},
		},
		{
			name: "patch state only",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				id := model.NewDeviceID()
				fake.PatchDeviceStub = func(_ context.Context, deviceID model.DeviceID, _ map[string]any) (*model.Device, error) {
					return &model.Device{
						ID:    deviceID,
						Name:  "Original",
						Brand: "Brand",
						State: model.StateInUse,
					}, nil
				}

				return id
			},
			updates: map[string]any{
				"state": "in_use",
			},
			expectError: false,
			validate: func(t *testing.T, d *model.Device) {
				require.Equal(t, model.StateInUse, d.State)
			},
		},
		{
			name: "device not found",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				fake.PatchDeviceReturns(nil, model.ErrDeviceNotFound)

				return model.NewDeviceID()
			},
			updates: map[string]any{
				"name": "New Name",
			},
			expectError: true,
			validate:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			deviceID := tc.setupSvc(svc)

			handler := commands.NewPatchDeviceCommandHandler(svc, log, tp, mc)

			cmd := commands.PatchDeviceCommand{
				ID:      deviceID,
				Updates: tc.updates,
			}

			device, err := handler.Handle(t.Context(), cmd)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, device)
				if tc.validate != nil {
					tc.validate(t, device)
				}
			}
		})
	}
}

func TestDeleteDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.NewTestLogger()
	tp := otelNoop.NewTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mocks.FakeDevicesService) model.DeviceID
		expectError bool
		expectedErr error
	}{
		{
			name: "successfully delete device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				id := model.NewDeviceID()
				fake.DeleteDeviceReturns(nil)

				return id
			},
			expectError: false,
		},
		{
			name: "cannot delete device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				fake.DeleteDeviceReturns(model.ErrDeviceNotFound)

				return model.NewDeviceID()
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.FakeDevicesService{}
			deviceID := tc.setupSvc(svc)

			handler := commands.NewDeleteDeviceCommandHandler(svc, log, tp, mc)

			cmd := commands.DeleteDeviceCommand{ID: deviceID}

			result, err := handler.Handle(t.Context(), cmd)

			if tc.expectError {
				require.Error(t, err)
				require.False(t, result.Success)
				if tc.expectedErr != nil {
					require.ErrorIs(t, err, tc.expectedErr)
				}
			} else {
				require.NoError(t, err)
				require.True(t, result.Success)
			}
		})
	}
}
