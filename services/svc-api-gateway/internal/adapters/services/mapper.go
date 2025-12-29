package services

import (
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
)

func toProtoState(s model.State) devicev1.DeviceState {
	switch s {
	case model.StateAvailable:
		return devicev1.DeviceState_DEVICE_STATE_AVAILABLE
	case model.StateInUse:
		return devicev1.DeviceState_DEVICE_STATE_IN_USE
	case model.StateInactive:
		return devicev1.DeviceState_DEVICE_STATE_INACTIVE
	default:
		return devicev1.DeviceState_DEVICE_STATE_UNSPECIFIED
	}
}

func toDomainState(s devicev1.DeviceState) model.State {
	switch s {
	case devicev1.DeviceState_DEVICE_STATE_AVAILABLE:
		return model.StateAvailable
	case devicev1.DeviceState_DEVICE_STATE_IN_USE:
		return model.StateInUse
	case devicev1.DeviceState_DEVICE_STATE_INACTIVE:
		return model.StateInactive
	default:
		return model.StateAvailable
	}
}

func toDomainDevice(d *devicev1.Device) *model.Device {
	if d == nil {
		return nil
	}

	id, _ := model.ParseDeviceID(d.GetId())

	device := &model.Device{
		ID:    id,
		Name:  d.GetName(),
		Brand: d.GetBrand(),
		State: toDomainState(d.GetState()),
	}

	if d.GetCreatedAt() != nil {
		device.CreatedAt = d.GetCreatedAt().AsTime()
	}

	if d.GetUpdatedAt() != nil {
		device.UpdatedAt = d.GetUpdatedAt().AsTime()
	}

	return device
}

func toDomainDevices(devices []*devicev1.Device) []*model.Device {
	result := make([]*model.Device, 0, len(devices))
	for _, d := range devices {
		result = append(result, toDomainDevice(d))
	}

	return result
}

func toDomainPagination(p *devicev1.Pagination) model.Pagination {
	if p == nil {
		return model.Pagination{}
	}

	return model.Pagination{
		Page:        uint(p.GetPage()),
		Size:        uint(p.GetSize()),
		TotalItems:  uint(p.GetTotalItems()),
		TotalPages:  uint(p.GetTotalPages()),
		HasNext:     p.GetHasNext(),
		HasPrevious: p.GetHasPrevious(),
	}
}

func toProtoListRequest(filter model.DeviceFilter) *devicev1.ListDevicesRequest {
	req := &devicev1.ListDevicesRequest{
		Page: uint32(filter.Page),
		Size: uint32(filter.Size),
		Sort: filter.Sort,
	}

	if len(filter.Brands) > 0 {
		req.Brands = filter.Brands
	}

	if len(filter.States) > 0 {
		for _, s := range filter.States {
			req.States = append(req.States, toProtoState(s))
		}
	}

	return req
}

func toProtoPatchRequest(id model.DeviceID, updates map[string]any) *devicev1.PatchDeviceRequest {
	req := &devicev1.PatchDeviceRequest{
		Id: id.String(),
	}

	if name, ok := updates["name"].(string); ok {
		req.Name = &name
	}

	if brand, ok := updates["brand"].(string); ok {
		req.Brand = &brand
	}

	if stateStr, ok := updates["state"].(string); ok {
		if state, err := model.ParseState(stateStr); err == nil {
			protoState := toProtoState(state)
			req.State = &protoState
		}
	}

	if state, ok := updates["state"].(model.State); ok {
		protoState := toProtoState(state)
		req.State = &protoState
	}

	return req
}
