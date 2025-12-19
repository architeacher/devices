package grpc

import (
	"context"
	"errors"

	"github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/commands"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/queries"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DevicesHandler struct {
	devicev1.UnimplementedDeviceServiceServer
	app *usecases.Application
}

func NewDevicesHandler(app *usecases.Application) *DevicesHandler {
	return &DevicesHandler{app: app}
}

func (h *DevicesHandler) CreateDevice(ctx context.Context, req *devicev1.CreateDeviceRequest) (*devicev1.CreateDeviceResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if req.Brand == "" {
		return nil, status.Error(codes.InvalidArgument, "brand is required")
	}

	cmd := commands.CreateDeviceCommand{
		Name:  req.Name,
		Brand: req.Brand,
		State: toDomainState(req.State),
	}

	device, err := h.app.Commands.CreateDevice.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &devicev1.CreateDeviceResponse{
		Device: toProtoDevice(device),
	}, nil
}

func (h *DevicesHandler) GetDevice(ctx context.Context, req *devicev1.GetDeviceRequest) (*devicev1.GetDeviceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := model.ParseDeviceID(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device ID")
	}

	query := queries.GetDeviceQuery{ID: id}

	device, err := h.app.Queries.GetDevice.Execute(ctx, query)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &devicev1.GetDeviceResponse{
		Device: toProtoDevice(device),
	}, nil
}

func (h *DevicesHandler) ListDevices(ctx context.Context, req *devicev1.ListDevicesRequest) (*devicev1.ListDevicesResponse, error) {
	filter := toDomainFilter(req)

	query := queries.ListDevicesQuery{Filter: filter}

	list, err := h.app.Queries.ListDevices.Execute(ctx, query)
	if err != nil {
		return nil, toGRPCError(err)
	}

	devices := make([]*devicev1.Device, len(list.Devices))
	for index, device := range list.Devices {
		devices[index] = toProtoDevice(device)
	}

	return &devicev1.ListDevicesResponse{
		Devices:    devices,
		Pagination: toProtoPagination(list.Pagination),
	}, nil
}

func (h *DevicesHandler) UpdateDevice(ctx context.Context, req *devicev1.UpdateDeviceRequest) (*devicev1.UpdateDeviceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := model.ParseDeviceID(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device ID")
	}

	cmd := commands.UpdateDeviceCommand{
		ID:    id,
		Name:  req.Name,
		Brand: req.Brand,
		State: toDomainState(req.State),
	}

	device, err := h.app.Commands.UpdateDevice.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &devicev1.UpdateDeviceResponse{
		Device: toProtoDevice(device),
	}, nil
}

func (h *DevicesHandler) PatchDevice(ctx context.Context, req *devicev1.PatchDeviceRequest) (*devicev1.PatchDeviceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := model.ParseDeviceID(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device ID")
	}

	updates := make(map[string]any)

	if req.Name != nil {
		updates["name"] = *req.Name
	}

	if req.Brand != nil {
		updates["brand"] = *req.Brand
	}

	if req.State != nil {
		updates["state"] = toDomainState(*req.State).String()
	}

	cmd := commands.PatchDeviceCommand{
		ID:      id,
		Updates: updates,
	}

	device, err := h.app.Commands.PatchDevice.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &devicev1.PatchDeviceResponse{
		Device: toProtoDevice(device),
	}, nil
}

func (h *DevicesHandler) DeleteDevice(ctx context.Context, req *devicev1.DeleteDeviceRequest) (*emptypb.Empty, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := model.ParseDeviceID(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device ID")
	}

	cmd := commands.DeleteDeviceCommand{ID: id}

	_, err = h.app.Commands.DeleteDevice.Handle(ctx, cmd)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &emptypb.Empty{}, nil
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, model.ErrDeviceNotFound):
		return status.Error(codes.NotFound, "device not found")
	case errors.Is(err, model.ErrCannotUpdateInUseDevice):
		return status.Error(codes.FailedPrecondition, "cannot update name or brand of in-use device")
	case errors.Is(err, model.ErrCannotDeleteInUseDevice):
		return status.Error(codes.FailedPrecondition, "cannot delete in-use device")
	case errors.Is(err, model.ErrDuplicateDevice):
		return status.Error(codes.AlreadyExists, "device already exists")
	case errors.Is(err, model.ErrInvalidState):
		return status.Error(codes.InvalidArgument, "invalid device state")
	case errors.Is(err, model.ErrInvalidDeviceID):
		return status.Error(codes.InvalidArgument, "invalid device ID")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
