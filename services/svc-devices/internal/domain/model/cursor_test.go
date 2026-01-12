package model_test

import (
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestCursorEncodeDecode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cursor model.Cursor
	}{
		{
			name: "timestamp field cursor",
			cursor: model.Cursor{
				Field:     "-createdAt",
				Value:     "2024-01-15T10:30:00.123456789Z",
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				Direction: model.CursorDirectionNext,
			},
		},
		{
			name: "string field cursor",
			cursor: model.Cursor{
				Field:     "name",
				Value:     "iPhone 15",
				ID:        "550e8400-e29b-41d4-a716-446655440001",
				Direction: model.CursorDirectionPrev,
			},
		},
		{
			name: "state field cursor",
			cursor: model.Cursor{
				Field:     "-state",
				Value:     "available",
				ID:        "550e8400-e29b-41d4-a716-446655440002",
				Direction: model.CursorDirectionNext,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := model.EncodeCursor(tc.cursor)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			decoded, err := model.DecodeCursor(encoded)
			require.NoError(t, err)
			require.Equal(t, tc.cursor.Field, decoded.Field)
			require.Equal(t, tc.cursor.ID, decoded.ID)
			require.Equal(t, tc.cursor.Direction, decoded.Direction)
		})
	}
}

func TestDecodeCursor_InvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		encoded string
	}{
		{
			name:    "empty string",
			encoded: "",
		},
		{
			name:    "invalid base64",
			encoded: "not-valid-base64!!!",
		},
		{
			name:    "valid base64 but invalid json",
			encoded: "bm90LWpzb24",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := model.DecodeCursor(tc.encoded)
			require.Error(t, err)
			require.ErrorIs(t, err, model.ErrInvalidCursor)
		})
	}
}

func TestNewCursorFromDevice(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	device := &model.Device{
		ID:        model.NewDeviceID(),
		Name:      "Galaxy S24",
		Brand:     "Samsung",
		State:     model.StateAvailable,
		CreatedAt: now,
		UpdatedAt: now,
	}

	cases := []struct {
		name          string
		sortField     string
		direction     model.CursorDirection
		expectedField string
	}{
		{
			name:          "created_at ascending",
			sortField:     "createdAt",
			direction:     model.CursorDirectionNext,
			expectedField: "createdAt",
		},
		{
			name:          "created_at descending",
			sortField:     "-createdAt",
			direction:     model.CursorDirectionPrev,
			expectedField: "-createdAt",
		},
		{
			name:          "name field",
			sortField:     "name",
			direction:     model.CursorDirectionNext,
			expectedField: "name",
		},
		{
			name:          "brand field descending",
			sortField:     "-brand",
			direction:     model.CursorDirectionNext,
			expectedField: "-brand",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cursor := model.NewCursorFromDevice(device, tc.sortField, tc.direction)

			require.Equal(t, tc.expectedField, cursor.Field)
			require.Equal(t, device.ID.String(), cursor.ID)
			require.Equal(t, tc.direction, cursor.Direction)
			require.NotNil(t, cursor.Value)
		})
	}
}

func TestCursor_ParseCursorValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		cursor      model.Cursor
		expectError bool
	}{
		{
			name: "valid timestamp value",
			cursor: model.Cursor{
				Field: "-createdAt",
				Value: "2024-01-15T10:30:00.123456789Z",
			},
			expectError: false,
		},
		{
			name: "invalid timestamp value",
			cursor: model.Cursor{
				Field: "-createdAt",
				Value: "not-a-timestamp",
			},
			expectError: true,
		},
		{
			name: "string value for name field",
			cursor: model.Cursor{
				Field: "name",
				Value: "iPhone 15",
			},
			expectError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			val, err := tc.cursor.ParseCursorValue()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, val)
			}
		})
	}
}
