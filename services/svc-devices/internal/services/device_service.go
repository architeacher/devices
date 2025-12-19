package services

import (
	"context"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
)

type DevicesService struct {
	repo ports.DeviceRepository
}

func NewDevicesService(repo ports.DeviceRepository) *DevicesService {
	return &DevicesService{repo: repo}
}

func (s *DevicesService) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	device := model.NewDevice(name, brand, state)

	if err := s.repo.Create(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *DevicesService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DevicesService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	return s.repo.List(ctx, filter)
}

func (s *DevicesService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := device.Update(name, brand, state); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *DevicesService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := device.Patch(updates); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *DevicesService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !device.CanDelete() {
		return model.ErrCannotDeleteInUseDevice
	}

	return s.repo.Delete(ctx, id)
}
