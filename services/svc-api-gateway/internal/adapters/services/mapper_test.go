package services

import (
	"testing"
	"time"

	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestToProtoState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    model.State
		expected devicev1.DeviceState
	}{
		{
			name:     "available state",
			input:    model.StateAvailable,
			expected: devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
		},
		{
			name:     "in-use state",
			input:    model.StateInUse,
			expected: devicev1.DeviceState_DEVICE_STATE_IN_USE,
		},
		{
			name:     "inactive state",
			input:    model.StateInactive,
			expected: devicev1.DeviceState_DEVICE_STATE_INACTIVE,
		},
		{
			name:     "unknown state defaults to unspecified",
			input:    model.State("unknown"),
			expected: devicev1.DeviceState_DEVICE_STATE_UNSPECIFIED,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toProtoState(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestToDomainState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    devicev1.DeviceState
		expected model.State
	}{
		{
			name:     "available state",
			input:    devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
			expected: model.StateAvailable,
		},
		{
			name:     "in-use state",
			input:    devicev1.DeviceState_DEVICE_STATE_IN_USE,
			expected: model.StateInUse,
		},
		{
			name:     "inactive state",
			input:    devicev1.DeviceState_DEVICE_STATE_INACTIVE,
			expected: model.StateInactive,
		},
		{
			name:     "unspecified state defaults to available",
			input:    devicev1.DeviceState_DEVICE_STATE_UNSPECIFIED,
			expected: model.StateAvailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toDomainState(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestToDomainDevice(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	testID := "123e4567-e89b-12d3-a456-426614174000"

	cases := []struct {
		name     string
		input    *devicev1.Device
		validate func(t *testing.T, result *model.Device)
	}{
		{
			name:  "nil device returns nil",
			input: nil,
			validate: func(t *testing.T, result *model.Device) {
				require.Nil(t, result)
			},
		},
		{
			name: "valid device",
			input: &devicev1.Device{
				Id:        testID,
				Name:      "iPhone 15",
				Brand:     "Apple",
				State:     devicev1.DeviceState_DEVICE_STATE_AVAILABLE,
				CreatedAt: timestamppb.New(now),
				UpdatedAt: timestamppb.New(now),
			},
			validate: func(t *testing.T, result *model.Device) {
				require.NotNil(t, result)
				require.Equal(t, testID, result.ID.String())
				require.Equal(t, "iPhone 15", result.Name)
				require.Equal(t, "Apple", result.Brand)
				require.Equal(t, model.StateAvailable, result.State)
				require.Equal(t, now.Unix(), result.CreatedAt.Unix())
				require.Equal(t, now.Unix(), result.UpdatedAt.Unix())
			},
		},
		{
			name: "device without timestamps",
			input: &devicev1.Device{
				Id:    testID,
				Name:  "Galaxy S24",
				Brand: "Samsung",
				State: devicev1.DeviceState_DEVICE_STATE_IN_USE,
			},
			validate: func(t *testing.T, result *model.Device) {
				require.NotNil(t, result)
				require.Equal(t, "Galaxy S24", result.Name)
				require.Equal(t, "Samsung", result.Brand)
				require.Equal(t, model.StateInUse, result.State)
				require.True(t, result.CreatedAt.IsZero())
				require.True(t, result.UpdatedAt.IsZero())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toDomainDevice(tc.input)
			tc.validate(t, result)
		})
	}
}

func TestToDomainDevices(t *testing.T) {
	t.Parallel()

	testID1 := "123e4567-e89b-12d3-a456-426614174000"
	testID2 := "123e4567-e89b-12d3-a456-426614174001"

	cases := []struct {
		name         string
		input        []*devicev1.Device
		expectedLen  int
		validateItem func(t *testing.T, idx int, device *model.Device)
	}{
		{
			name:        "empty slice",
			input:       []*devicev1.Device{},
			expectedLen: 0,
			validateItem: func(t *testing.T, idx int, device *model.Device) {
				t.Fatal("should not be called for empty slice")
			},
		},
		{
			name: "multiple devices",
			input: []*devicev1.Device{
				{Id: testID1, Name: "Device 1", Brand: "Brand 1", State: devicev1.DeviceState_DEVICE_STATE_AVAILABLE},
				{Id: testID2, Name: "Device 2", Brand: "Brand 2", State: devicev1.DeviceState_DEVICE_STATE_IN_USE},
			},
			expectedLen: 2,
			validateItem: func(t *testing.T, idx int, device *model.Device) {
				switch idx {
				case 0:
					require.Equal(t, testID1, device.ID.String())
					require.Equal(t, "Device 1", device.Name)
				case 1:
					require.Equal(t, testID2, device.ID.String())
					require.Equal(t, "Device 2", device.Name)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toDomainDevices(tc.input)
			require.Len(t, result, tc.expectedLen)

			for idx, device := range result {
				tc.validateItem(t, idx, device)
			}
		})
	}
}

func TestToDomainPagination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    *devicev1.Pagination
		expected model.Pagination
	}{
		{
			name:     "nil pagination returns zero value",
			input:    nil,
			expected: model.Pagination{},
		},
		{
			name: "valid pagination",
			input: &devicev1.Pagination{
				Page:        1,
				Size:        20,
				TotalItems:  100,
				TotalPages:  5,
				HasNext:     true,
				HasPrevious: false,
			},
			expected: model.Pagination{
				Page:        1,
				Size:        20,
				TotalItems:  100,
				TotalPages:  5,
				HasNext:     true,
				HasPrevious: false,
			},
		},
		{
			name: "last page pagination",
			input: &devicev1.Pagination{
				Page:        5,
				Size:        20,
				TotalItems:  100,
				TotalPages:  5,
				HasNext:     false,
				HasPrevious: true,
			},
			expected: model.Pagination{
				Page:        5,
				Size:        20,
				TotalItems:  100,
				TotalPages:  5,
				HasNext:     false,
				HasPrevious: true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toDomainPagination(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestToProtoListRequest(t *testing.T) {
	t.Parallel()

	brand := "Apple"
	state := model.StateAvailable

	cases := []struct {
		name     string
		input    model.DeviceFilter
		validate func(t *testing.T, req *devicev1.ListDevicesRequest)
	}{
		{
			name: "empty filter with defaults",
			input: model.DeviceFilter{
				Page: 1,
				Size: 20,
				Sort: "-createdAt",
			},
			validate: func(t *testing.T, req *devicev1.ListDevicesRequest) {
				require.Equal(t, uint32(1), req.GetPage())
				require.Equal(t, uint32(20), req.GetSize())
				require.Equal(t, "-createdAt", req.GetSort())
				require.Nil(t, req.Brand)
				require.Nil(t, req.State)
			},
		},
		{
			name: "filter with brand",
			input: model.DeviceFilter{
				Brand: &brand,
				Page:  1,
				Size:  10,
			},
			validate: func(t *testing.T, req *devicev1.ListDevicesRequest) {
				require.NotNil(t, req.Brand)
				require.Equal(t, "Apple", *req.Brand)
			},
		},
		{
			name: "filter with state",
			input: model.DeviceFilter{
				State: &state,
				Page:  1,
				Size:  10,
			},
			validate: func(t *testing.T, req *devicev1.ListDevicesRequest) {
				require.NotNil(t, req.State)
				require.Equal(t, devicev1.DeviceState_DEVICE_STATE_AVAILABLE, *req.State)
			},
		},
		{
			name: "filter with all options",
			input: model.DeviceFilter{
				Brand: &brand,
				State: &state,
				Page:  2,
				Size:  50,
				Sort:  "name",
			},
			validate: func(t *testing.T, req *devicev1.ListDevicesRequest) {
				require.NotNil(t, req.Brand)
				require.Equal(t, "Apple", *req.Brand)
				require.NotNil(t, req.State)
				require.Equal(t, devicev1.DeviceState_DEVICE_STATE_AVAILABLE, *req.State)
				require.Equal(t, uint32(2), req.GetPage())
				require.Equal(t, uint32(50), req.GetSize())
				require.Equal(t, "name", req.GetSort())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toProtoListRequest(tc.input)
			tc.validate(t, result)
		})
	}
}

func TestToProtoPatchRequest(t *testing.T) {
	t.Parallel()

	testID, _ := model.ParseDeviceID("123e4567-e89b-12d3-a456-426614174000")

	cases := []struct {
		name     string
		id       model.DeviceID
		updates  map[string]any
		validate func(t *testing.T, req *devicev1.PatchDeviceRequest)
	}{
		{
			name:    "empty updates",
			id:      testID,
			updates: map[string]any{},
			validate: func(t *testing.T, req *devicev1.PatchDeviceRequest) {
				require.Equal(t, testID.String(), req.GetId())
				require.Nil(t, req.Name)
				require.Nil(t, req.Brand)
				require.Nil(t, req.State)
			},
		},
		{
			name: "update name only",
			id:   testID,
			updates: map[string]any{
				"name": "New Name",
			},
			validate: func(t *testing.T, req *devicev1.PatchDeviceRequest) {
				require.NotNil(t, req.Name)
				require.Equal(t, "New Name", *req.Name)
				require.Nil(t, req.Brand)
				require.Nil(t, req.State)
			},
		},
		{
			name: "update brand only",
			id:   testID,
			updates: map[string]any{
				"brand": "New Brand",
			},
			validate: func(t *testing.T, req *devicev1.PatchDeviceRequest) {
				require.Nil(t, req.Name)
				require.NotNil(t, req.Brand)
				require.Equal(t, "New Brand", *req.Brand)
				require.Nil(t, req.State)
			},
		},
		{
			name: "update state with string",
			id:   testID,
			updates: map[string]any{
				"state": "in-use",
			},
			validate: func(t *testing.T, req *devicev1.PatchDeviceRequest) {
				require.NotNil(t, req.State)
				require.Equal(t, devicev1.DeviceState_DEVICE_STATE_IN_USE, *req.State)
			},
		},
		{
			name: "update state with model.State",
			id:   testID,
			updates: map[string]any{
				"state": model.StateInactive,
			},
			validate: func(t *testing.T, req *devicev1.PatchDeviceRequest) {
				require.NotNil(t, req.State)
				require.Equal(t, devicev1.DeviceState_DEVICE_STATE_INACTIVE, *req.State)
			},
		},
		{
			name: "update all fields",
			id:   testID,
			updates: map[string]any{
				"name":  "Updated Name",
				"brand": "Updated Brand",
				"state": "available",
			},
			validate: func(t *testing.T, req *devicev1.PatchDeviceRequest) {
				require.NotNil(t, req.Name)
				require.Equal(t, "Updated Name", *req.Name)
				require.NotNil(t, req.Brand)
				require.Equal(t, "Updated Brand", *req.Brand)
				require.NotNil(t, req.State)
				require.Equal(t, devicev1.DeviceState_DEVICE_STATE_AVAILABLE, *req.State)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toProtoPatchRequest(tc.id, tc.updates)
			tc.validate(t, result)
		})
	}
}
