package model_test

import (
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DeviceTestSuite struct {
	suite.Suite
}

func TestDeviceTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DeviceTestSuite))
}

func (s *DeviceTestSuite) TestNewDeviceID() {
	s.T().Parallel()

	id := model.NewDeviceID()

	s.Require().False(id.IsZero())
	s.Require().NotEmpty(id.String())
}

func (s *DeviceTestSuite) TestParseDeviceID() {
	s.T().Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			input:   "01234567-89ab-cdef-0123-456789abcdef",
			wantErr: false,
		},
		{
			name:    "invalid UUID",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			id, err := model.ParseDeviceID(tc.input)

			if tc.wantErr {
				s.Require().Error(err)

				return
			}

			s.Require().NoError(err)
			s.Require().Equal(tc.input, id.String())
		})
	}
}

func (s *DeviceTestSuite) TestDeviceID_IsZero() {
	s.T().Parallel()

	s.Run("new ID is not zero", func() {
		id := model.NewDeviceID()
		s.Require().False(id.IsZero())
	})

	s.Run("zero value is zero", func() {
		var id model.DeviceID
		s.Require().True(id.IsZero())
	})
}

func (s *DeviceTestSuite) TestNewDevice() {
	s.T().Parallel()

	name := "Test Device"
	brand := "Test Brand"
	state := model.StateAvailable

	device := model.NewDevice(name, brand, state)

	s.Require().NotNil(device)
	s.Require().False(device.ID.IsZero())
	s.Require().Equal(name, device.Name)
	s.Require().Equal(brand, device.Brand)
	s.Require().Equal(state, device.State)
	s.Require().False(device.CreatedAt.IsZero())
	s.Require().False(device.UpdatedAt.IsZero())
}

func (s *DeviceTestSuite) TestDevice_CanUpdateNameAndBrand() {
	s.T().Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected bool
	}{
		{
			name:     "available device can update",
			state:    model.StateAvailable,
			expected: true,
		},
		{
			name:     "in-use device cannot update",
			state:    model.StateInUse,
			expected: false,
		},
		{
			name:     "inactive device can update",
			state:    model.StateInactive,
			expected: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			device := model.NewDevice("Test", "Brand", tc.state)
			s.Require().Equal(tc.expected, device.CanUpdateNameAndBrand())
		})
	}
}

func (s *DeviceTestSuite) TestDevice_CanDelete() {
	s.T().Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected bool
	}{
		{
			name:     "available device can delete",
			state:    model.StateAvailable,
			expected: true,
		},
		{
			name:     "in-use device cannot delete",
			state:    model.StateInUse,
			expected: false,
		},
		{
			name:     "inactive device can delete",
			state:    model.StateInactive,
			expected: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			device := model.NewDevice("Test", "Brand", tc.state)
			s.Require().Equal(tc.expected, device.CanDelete())
		})
	}
}

func (s *DeviceTestSuite) TestDevice_Update() {
	s.T().Parallel()

	s.Run("update available device succeeds", func() {
		device := model.NewDevice("Old Name", "Old Brand", model.StateAvailable)
		originalUpdatedAt := device.UpdatedAt

		time.Sleep(time.Millisecond)

		err := device.Update("New Name", "New Brand", model.StateInactive)

		s.Require().NoError(err)
		s.Require().Equal("New Name", device.Name)
		s.Require().Equal("New Brand", device.Brand)
		s.Require().Equal(model.StateInactive, device.State)
		s.Require().True(device.UpdatedAt.After(originalUpdatedAt))
	})

	s.Run("update in-use device name fails", func() {
		device := model.NewDevice("Old Name", "Old Brand", model.StateInUse)

		err := device.Update("New Name", "Old Brand", model.StateInUse)

		s.Require().ErrorIs(err, model.ErrCannotUpdateInUseDevice)
	})

	s.Run("update in-use device brand fails", func() {
		device := model.NewDevice("Old Name", "Old Brand", model.StateInUse)

		err := device.Update("Old Name", "New Brand", model.StateInUse)

		s.Require().ErrorIs(err, model.ErrCannotUpdateInUseDevice)
	})

	s.Run("update in-use device state only succeeds", func() {
		device := model.NewDevice("Old Name", "Old Brand", model.StateInUse)

		err := device.Update("Old Name", "Old Brand", model.StateAvailable)

		s.Require().NoError(err)
		s.Require().Equal(model.StateAvailable, device.State)
	})
}

func (s *DeviceTestSuite) TestDefaultDeviceFilter() {
	s.T().Parallel()

	filter := model.DefaultDeviceFilter()

	s.Require().Equal(uint(1), filter.Page)
	s.Require().Equal(uint(20), filter.Size)
	s.Require().Equal([]string{"-createdAt"}, filter.Sort)
	s.Require().Empty(filter.Brands)
	s.Require().Empty(filter.States)
}

type StateTestSuite struct {
	suite.Suite
}

func TestStateTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(StateTestSuite))
}

func (s *StateTestSuite) TestParseState() {
	s.T().Parallel()

	cases := []struct {
		name     string
		input    string
		expected model.State
		wantErr  bool
	}{
		{
			name:     "available",
			input:    "available",
			expected: model.StateAvailable,
			wantErr:  false,
		},
		{
			name:     "in-use",
			input:    "in-use",
			expected: model.StateInUse,
			wantErr:  false,
		},
		{
			name:     "inactive",
			input:    "inactive",
			expected: model.StateInactive,
			wantErr:  false,
		},
		{
			name:     "invalid",
			input:    "invalid",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			state, err := model.ParseState(tc.input)

			if tc.wantErr {
				s.Require().Error(err)

				return
			}

			s.Require().NoError(err)
			s.Require().Equal(tc.expected, state)
		})
	}
}

func (s *StateTestSuite) TestState_String() {
	s.T().Parallel()

	cases := []struct {
		state    model.State
		expected string
	}{
		{model.StateAvailable, "available"},
		{model.StateInUse, "in-use"},
		{model.StateInactive, "inactive"},
	}

	for _, tc := range cases {
		s.Run(tc.expected, func() {
			s.Require().Equal(tc.expected, tc.state.String())
		})
	}
}

func (s *StateTestSuite) TestState_IsValid() {
	s.T().Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected bool
	}{
		{"available is valid", model.StateAvailable, true},
		{"in-use is valid", model.StateInUse, true},
		{"inactive is valid", model.StateInactive, true},
		{"invalid state", model.State("invalid"), false},
		{"empty state", model.State(""), false},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Require().Equal(tc.expected, tc.state.IsValid())
		})
	}
}

func (s *StateTestSuite) TestAllStates() {
	s.T().Parallel()

	states := model.AllStates()

	s.Require().Len(states, 3)
	s.Require().Contains(states, model.StateAvailable)
	s.Require().Contains(states, model.StateInUse)
	s.Require().Contains(states, model.StateInactive)
}

func TestDeviceID_Parallel(t *testing.T) {
	t.Parallel()

	t.Run("NewDeviceID generates unique IDs", func(t *testing.T) {
		t.Parallel()

		ids := make(map[string]struct{})
		for index := 0; index < 100; index++ {
			id := model.NewDeviceID()
			_, exists := ids[id.String()]
			require.False(t, exists, "duplicate ID generated")
			ids[id.String()] = struct{}{}
		}
	})
}
