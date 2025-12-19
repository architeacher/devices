package model_test

import (
	"testing"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestState_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected string
	}{
		{
			name:     "available state returns correct string",
			state:    model.StateAvailable,
			expected: "available",
		},
		{
			name:     "in-use state returns correct string",
			state:    model.StateInUse,
			expected: "in-use",
		},
		{
			name:     "inactive state returns correct string",
			state:    model.StateInactive,
			expected: "inactive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.state.String())
		})
	}
}

func TestState_IsValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		state    model.State
		expected bool
	}{
		{
			name:     "available is valid",
			state:    model.StateAvailable,
			expected: true,
		},
		{
			name:     "in-use is valid",
			state:    model.StateInUse,
			expected: true,
		},
		{
			name:     "inactive is valid",
			state:    model.StateInactive,
			expected: true,
		},
		{
			name:     "empty string is invalid",
			state:    model.State(""),
			expected: false,
		},
		{
			name:     "unknown state is invalid",
			state:    model.State("unknown"),
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.state.IsValid())
		})
	}
}

func TestParseState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		input         string
		expectedState model.State
		expectError   bool
	}{
		{
			name:          "parse available",
			input:         "available",
			expectedState: model.StateAvailable,
			expectError:   false,
		},
		{
			name:          "parse Available with uppercase",
			input:         "Available",
			expectedState: model.StateAvailable,
			expectError:   false,
		},
		{
			name:          "parse in-use",
			input:         "in-use",
			expectedState: model.StateInUse,
			expectError:   false,
		},
		{
			name:          "parse inactive",
			input:         "inactive",
			expectedState: model.StateInactive,
			expectError:   false,
		},
		{
			name:          "parse with whitespace",
			input:         "  available  ",
			expectedState: model.StateAvailable,
			expectError:   false,
		},
		{
			name:          "parse invalid state",
			input:         "invalid",
			expectedState: "",
			expectError:   true,
		},
		{
			name:          "parse empty string",
			input:         "",
			expectedState: "",
			expectError:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state, err := model.ParseState(tc.input)

			if tc.expectError {
				require.Error(t, err)
				require.Empty(t, state)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedState, state)
			}
		})
	}
}

func TestAllStates(t *testing.T) {
	t.Parallel()

	states := model.AllStates()

	require.Len(t, states, 3)
	require.Contains(t, states, model.StateAvailable)
	require.Contains(t, states, model.StateInUse)
	require.Contains(t, states, model.StateInactive)
}
