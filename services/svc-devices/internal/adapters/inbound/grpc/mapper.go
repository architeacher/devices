package grpc

import (
	"github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoDevice(d *model.Device) *devicev1.Device {
	return &devicev1.Device{
		Id:        d.ID.String(),
		Name:      d.Name,
		Brand:     d.Brand,
		State:     toProtoState(d.State),
		CreatedAt: timestamppb.New(d.CreatedAt),
		UpdatedAt: timestamppb.New(d.UpdatedAt),
	}
}

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

func toProtoPagination(p model.Pagination) *devicev1.Pagination {
	return &devicev1.Pagination{
		Page:        uint32(p.Page),
		Size:        uint32(p.Size),
		TotalItems:  uint32(p.TotalItems),
		TotalPages:  uint32(p.TotalPages),
		HasNext:     p.HasNext,
		HasPrevious: p.HasPrevious,
	}
}

func toDomainFilter(req *devicev1.ListDevicesRequest) model.DeviceFilter {
	filter := model.DefaultDeviceFilter()

	if len(req.GetBrands()) > 0 {
		filter.Brands = req.GetBrands()
	}

	if len(req.GetStates()) > 0 {
		states := make([]model.State, 0, len(req.GetStates()))
		for _, s := range req.GetStates() {
			states = append(states, toDomainState(s))
		}
		filter.States = states
	}

	if req.Page > 0 {
		filter.Page = uint(req.Page)
	}

	if req.Size > 0 {
		filter.Size = uint(req.Size)
	}

	if len(req.GetSort()) > 0 {
		filter.Sort = req.GetSort()
	}

	return filter
}
