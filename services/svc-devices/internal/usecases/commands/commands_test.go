package commands_test

import (
	"context"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/commands"
	"github.com/stretchr/testify/require"
)

type mockDevicesService struct {
	createDeviceFn func(ctx context.Context, name, brand string, state model.State) (*model.Device, error)
	getDeviceFn    func(ctx context.Context, id model.DeviceID) (*model.Device, error)
	listDevicesFn  func(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
	updateDeviceFn func(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error)
	patchDeviceFn  func(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error)
	deleteDeviceFn func(ctx context.Context, id model.DeviceID) error
}

func newMockDevicesService() *mockDevicesService {
	return &mockDevicesService{}
}

func (m *mockDevicesService) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(ctx, name, brand, state)
	}

	return model.NewDevice(name, brand, state), nil
}

func (m *mockDevicesService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}

	return nil, model.ErrDeviceNotFound
}

func (m *mockDevicesService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	if m.listDevicesFn != nil {
		return m.listDevicesFn(ctx, filter)
	}

	return &model.DeviceList{
		Devices: []*model.Device{},
		Pagination: model.Pagination{
			Page:       filter.Page,
			Size:       filter.Size,
			TotalItems: 0,
			TotalPages: 1,
		},
		Filters: filter,
	}, nil
}

func (m *mockDevicesService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(ctx, id, name, brand, state)
	}

	return nil, model.ErrDeviceNotFound
}

func (m *mockDevicesService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	if m.patchDeviceFn != nil {
		return m.patchDeviceFn(ctx, id, updates)
	}

	return nil, model.ErrDeviceNotFound
}

func (m *mockDevicesService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id)
	}

	return model.ErrDeviceNotFound
}

func TestCreateDeviceCommandHandler(t *testing.T) {
	t.Parallel()

	log := logger.New("debug", "console")
	tp := telemetry.NewNoopTracerProvider()
	mc := noop.NewMetricsClient()

	cases := []struct {
		name        string
		cmd         commands.CreateDeviceCommand
		setupSvc    func(*mockDevicesService)
		expectError bool
	}{
		{
			name: "successfully create device",
			cmd: commands.CreateDeviceCommand{
				Name:  "Test Device",
				Brand: "Test Brand",
				State: model.StateAvailable,
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
			setupSvc: func(m *mockDevicesService) {
				m.createDeviceFn = func(_ context.Context, _, _ string, _ model.State) (*model.Device, error) {
					return nil, model.ErrDuplicateDevice
				}
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			if tc.setupSvc != nil {
				tc.setupSvc(svc)
			}

			handler := commands.NewCreateDeviceCommandHandler(svc, log, tp, mc)
			ctx := context.Background()

			device, err := handler.Handle(ctx, tc.cmd)

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
		setupSvc    func(*mockDevicesService) model.DeviceID
		newName     string
		newBrand    string
		newState    model.State
		expectError bool
	}{
		{
			name: "successfully update available device",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Original Brand", model.StateAvailable)
				m.updateDeviceFn = func(_ context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
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
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Original Brand", model.StateInUse)
				m.updateDeviceFn = func(_ context.Context, _ model.DeviceID, _, _ string, _ model.State) (*model.Device, error) {
					return nil, model.ErrCannotUpdateInUseDevice
				}

				return device.ID
			},
			newName:     "New Name",
			newBrand:    "Original Brand",
			newState:    model.StateInUse,
			expectError: true,
		},
		{
			name: "device not found",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				m.updateDeviceFn = func(_ context.Context, _ model.DeviceID, _, _ string, _ model.State) (*model.Device, error) {
					return nil, model.ErrDeviceNotFound
				}

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

			svc := newMockDevicesService()
			deviceID := tc.setupSvc(svc)

			handler := commands.NewUpdateDeviceCommandHandler(svc, log, tp, mc)
			ctx := context.Background()

			cmd := commands.UpdateDeviceCommand{
				ID:    deviceID,
				Name:  tc.newName,
				Brand: tc.newBrand,
				State: tc.newState,
			}

			device, err := handler.Handle(ctx, cmd)

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
		setupSvc    func(*mockDevicesService) model.DeviceID
		expectError bool
		expectedErr error
	}{
		{
			name: "successfully delete available device",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Test", "Brand", model.StateAvailable)
				m.deleteDeviceFn = func(_ context.Context, _ model.DeviceID) error {
					return nil
				}

				return device.ID
			},
			expectError: false,
		},
		{
			name: "cannot delete in-use device",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Test", "Brand", model.StateInUse)
				m.deleteDeviceFn = func(_ context.Context, _ model.DeviceID) error {
					return model.ErrCannotDeleteInUseDevice
				}

				return device.ID
			},
			expectError: true,
			expectedErr: model.ErrCannotDeleteInUseDevice,
		},
		{
			name: "device not found",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				m.deleteDeviceFn = func(_ context.Context, _ model.DeviceID) error {
					return model.ErrDeviceNotFound
				}

				return model.NewDeviceID()
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockDevicesService()
			deviceID := tc.setupSvc(svc)

			handler := commands.NewDeleteDeviceCommandHandler(svc, log, tp, mc)
			ctx := context.Background()

			cmd := commands.DeleteDeviceCommand{ID: deviceID}

			_, err := handler.Handle(ctx, cmd)

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
		setupSvc    func(*mockDevicesService) model.DeviceID
		updates     map[string]any
		expectError bool
		validate    func(*testing.T, *model.Device)
	}{
		{
			name: "patch name of available device",
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Brand", model.StateAvailable)
				m.patchDeviceFn = func(_ context.Context, id model.DeviceID, _ map[string]any) (*model.Device, error) {
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
			setupSvc: func(m *mockDevicesService) model.DeviceID {
				device := model.NewDevice("Original", "Brand", model.StateInUse)
				m.patchDeviceFn = func(_ context.Context, id model.DeviceID, _ map[string]any) (*model.Device, error) {
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

			svc := newMockDevicesService()
			deviceID := tc.setupSvc(svc)

			handler := commands.NewPatchDeviceCommandHandler(svc, log, tp, mc)
			ctx := context.Background()

			cmd := commands.PatchDeviceCommand{
				ID:      deviceID,
				Updates: tc.updates,
			}

			device, err := handler.Handle(ctx, cmd)

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
