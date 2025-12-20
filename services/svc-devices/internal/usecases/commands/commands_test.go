package commands_test

import (
	"context"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-devices/internal/mocks"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/commands"
	"github.com/stretchr/testify/require"
)

func TestCreateDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		cmd         commands.CreateDeviceCommand
		setupSvc    func(*mocks.FakeDevicesService)
		expectError bool
	}{
		{
			name: "successfully create device",
			cmd: commands.CreateDeviceCommand{
				Name:  "Test Device",
				Brand: "Test Brand",
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
			name: "create device with duplicate error",
			cmd: commands.CreateDeviceCommand{
				Name:  "Duplicate Device",
				Brand: "Test Brand",
				State: model.StateAvailable,
			},
			setupSvc: func(fake *mocks.FakeDevicesService) {
				fake.CreateDeviceReturns(nil, model.ErrDuplicateDevice)
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
				require.False(t, device.ID.IsZero())
			}
		})
	}
}

func TestUpdateDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
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
			name: "successfully update available device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Original Brand", model.StateAvailable)
				fake.UpdateDeviceStub = func(_ context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
					return &model.Device{
						ID:    id,
						Name:  name,
						Brand: brand,
						State: state,
					}, nil
				}

				return device.ID
			},
			newName:     "Updated Name",
			newBrand:    "Updated Brand",
			newState:    model.StateInUse,
			expectError: false,
		},
		{
			name: "cannot update name of in-use device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Original Brand", model.StateInUse)
				fake.UpdateDeviceReturns(nil, model.ErrCannotUpdateInUseDevice)

				return device.ID
			},
			newName:     "New Name",
			newBrand:    "Original Brand",
			newState:    model.StateInUse,
			expectError: true,
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
				require.Equal(t, tc.newName, device.Name)
				require.Equal(t, tc.newBrand, device.Brand)
				require.Equal(t, tc.newState, device.State)
			}
		})
	}
}

func TestDeleteDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mocks.FakeDevicesService) model.DeviceID
		expectError bool
		expectedErr error
	}{
		{
			name: "successfully delete available device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				fake.DeleteDeviceReturns(nil)

				return device.ID
			},
			expectError: false,
		},
		{
			name: "cannot delete in-use device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Test", "Brand", model.StateInUse)
				fake.DeleteDeviceReturns(model.ErrCannotDeleteInUseDevice)

				return device.ID
			},
			expectError: true,
			expectedErr: model.ErrCannotDeleteInUseDevice,
		},
		{
			name: "device not found",
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

			_, err := handler.Handle(t.Context(), cmd)

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedErr != nil {
					require.ErrorIs(t, err, tc.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPatchDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		setupSvc    func(*mocks.FakeDevicesService) model.DeviceID
		updates     map[string]any
		expectError bool
		validate    func(*testing.T, *model.Device)
	}{
		{
			name: "patch name of available device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Brand", model.StateAvailable)
				fake.PatchDeviceStub = func(_ context.Context, id model.DeviceID, _ map[string]any) (*model.Device, error) {
					return &model.Device{
						ID:    id,
						Name:  "Patched Name",
						Brand: "Brand",
						State: model.StateAvailable,
					}, nil
				}

				return device.ID
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
			name: "patch state only of in-use device",
			setupSvc: func(fake *mocks.FakeDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Brand", model.StateInUse)
				fake.PatchDeviceStub = func(_ context.Context, id model.DeviceID, _ map[string]any) (*model.Device, error) {
					return &model.Device{
						ID:    id,
						Name:  "Original",
						Brand: "Brand",
						State: model.StateAvailable,
					}, nil
				}

				return device.ID
			},
			updates: map[string]any{
				"state": "available",
			},
			expectError: false,
			validate: func(t *testing.T, d *model.Device) {
				require.Equal(t, model.StateAvailable, d.State)
			},
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
				tc.validate(t, device)
			}
		})
	}
}
