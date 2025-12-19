package model_test

import (
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewDeviceID(t *testing.T) {
	t.Parallel()

	id := model.NewDeviceID()

	require.False(t, id.IsZero())
	require.NotEqual(t, uuid.Nil, id.UUID)
}

func TestParseDeviceID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid UUID",
			input:       "019426d2-5b1e-7c8a-9f3e-123456789abc",
			expectError: false,
		},
		{
			name:        "invalid UUID",
			input:       "not-a-uuid",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			id, err := model.ParseDeviceID(tc.input)

			if tc.expectError {
				require.Error(t, err)
				require.True(t, id.IsZero())
			} else {
				require.NoError(t, err)
				require.False(t, id.IsZero())
				require.Equal(t, tc.input, id.String())
			}
		})
	}
}

func TestDeviceID_String(t *testing.T) {
	t.Parallel()

	expectedID := "019426d2-5b1e-7c8a-9f3e-123456789abc"
	id, err := model.ParseDeviceID(expectedID)

	require.NoError(t, err)
	require.Equal(t, expectedID, id.String())
}

func TestDeviceID_IsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		id       model.DeviceID
		expected bool
	}{
		{
			name:     "zero UUID is zero",
			id:       model.DeviceID{UUID: uuid.Nil},
			expected: true,
		},
		{
			name:     "new device ID is not zero",
			id:       model.NewDeviceID(),
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.id.IsZero())
		})
	}
}

func TestNewDevice(t *testing.T) {
	t.Parallel()

	name := "Test Device"
	brand := "Test Brand"
	state := model.StateAvailable

	beforeCreation := time.Now().UTC()
	device := model.NewDevice(name, brand, state)
	afterCreation := time.Now().UTC()

	require.NotNil(t, device)
	require.False(t, device.ID.IsZero())
	require.Equal(t, name, device.Name)
	require.Equal(t, brand, device.Brand)
	require.Equal(t, state, device.State)
	require.True(t, device.CreatedAt.After(beforeCreation) || device.CreatedAt.Equal(beforeCreation))
	require.True(t, device.CreatedAt.Before(afterCreation) || device.CreatedAt.Equal(afterCreation))
	require.Equal(t, device.CreatedAt, device.UpdatedAt)
}

func TestDevice_CanUpdateNameAndBrand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected bool
	}{
		{
			name:     "available device can be updated",
			state:    model.StateAvailable,
			expected: true,
		},
		{
			name:     "in-use device cannot be updated",
			state:    model.StateInUse,
			expected: false,
		},
		{
			name:     "inactive device can be updated",
			state:    model.StateInactive,
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			device := model.NewDevice("Test", "Brand", tc.state)

			require.Equal(t, tc.expected, device.CanUpdateNameAndBrand())
		})
	}
}

func TestDevice_CanDelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected bool
	}{
		{
			name:     "available device can be deleted",
			state:    model.StateAvailable,
			expected: true,
		},
		{
			name:     "in-use device cannot be deleted",
			state:    model.StateInUse,
			expected: false,
		},
		{
			name:     "inactive device can be deleted",
			state:    model.StateInactive,
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			device := model.NewDevice("Test", "Brand", tc.state)

			require.Equal(t, tc.expected, device.CanDelete())
		})
	}
}

func TestDevice_Update(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		initialName string
		initialState model.State
		newName     string
		newBrand    string
		newState    model.State
		expectError bool
	}{
		{
			name:         "update available device",
			initialName:  "Old Name",
			initialState: model.StateAvailable,
			newName:      "New Name",
			newBrand:     "New Brand",
			newState:     model.StateInUse,
			expectError:  false,
		},
		{
			name:         "update inactive device",
			initialName:  "Old Name",
			initialState: model.StateInactive,
			newName:      "New Name",
			newBrand:     "New Brand",
			newState:     model.StateAvailable,
			expectError:  false,
		},
		{
			name:         "cannot update name of in-use device",
			initialName:  "Old Name",
			initialState: model.StateInUse,
			newName:      "New Name",
			newBrand:     "Old Brand",
			newState:     model.StateInUse,
			expectError:  true,
		},
		{
			name:         "can update state only of in-use device",
			initialName:  "Same Name",
			initialState: model.StateInUse,
			newName:      "Same Name",
			newBrand:     "Same Brand",
			newState:     model.StateAvailable,
			expectError:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			device := model.NewDevice(tc.initialName, "Old Brand", tc.initialState)

			if tc.name == "can update state only of in-use device" {
				device.Brand = "Same Brand"
			}

			originalUpdatedAt := device.UpdatedAt
			time.Sleep(1 * time.Millisecond)

			err := device.Update(tc.newName, tc.newBrand, tc.newState)

			if tc.expectError {
				require.Error(t, err)
				require.ErrorIs(t, err, model.ErrCannotUpdateInUseDevice)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.newName, device.Name)
				require.Equal(t, tc.newBrand, device.Brand)
				require.Equal(t, tc.newState, device.State)
				require.True(t, device.UpdatedAt.After(originalUpdatedAt))
			}
		})
	}
}

func TestDevice_Patch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		initialState model.State
		updates      map[string]any
		expectError  bool
		expectedName string
	}{
		{
			name:         "patch name of available device",
			initialState: model.StateAvailable,
			updates:      map[string]any{"name": "New Name"},
			expectError:  false,
			expectedName: "New Name",
		},
		{
			name:         "patch state only of in-use device",
			initialState: model.StateInUse,
			updates:      map[string]any{"state": "available"},
			expectError:  false,
			expectedName: "Test",
		},
		{
			name:         "cannot patch name of in-use device",
			initialState: model.StateInUse,
			updates:      map[string]any{"name": "New Name"},
			expectError:  true,
			expectedName: "Test",
		},
		{
			name:         "invalid state returns error",
			initialState: model.StateAvailable,
			updates:      map[string]any{"state": "invalid"},
			expectError:  true,
			expectedName: "Test",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			device := model.NewDevice("Test", "Brand", tc.initialState)

			err := device.Patch(tc.updates)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedName, device.Name)
			}
		})
	}
}

func TestDefaultDeviceFilter(t *testing.T) {
	t.Parallel()

	filter := model.DefaultDeviceFilter()

	require.Equal(t, uint(1), filter.Page)
	require.Equal(t, uint(20), filter.Size)
	require.Equal(t, "-createdAt", filter.Sort)
	require.Nil(t, filter.Brand)
	require.Nil(t, filter.State)
}
