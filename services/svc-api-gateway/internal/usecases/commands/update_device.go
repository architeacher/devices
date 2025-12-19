package commands

import (
	"context"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/shared/decorator"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	UpdateDeviceCommand struct {
		ID    model.DeviceID
		Name  string
		Brand string
		State model.State
	}

	UpdateDeviceCommandHandler = decorator.CommandHandler[UpdateDeviceCommand, *model.Device]

	updateDeviceCommandHandler struct {
		deviceService ports.DevicesService
	}
)

func NewUpdateDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) UpdateDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[UpdateDeviceCommand, *model.Device](
		updateDeviceCommandHandler{deviceService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h updateDeviceCommandHandler) Handle(ctx context.Context, cmd UpdateDeviceCommand) (*model.Device, error) {
	return h.deviceService.UpdateDevice(ctx, cmd.ID, cmd.Name, cmd.Brand, cmd.State)
}

type (
	PatchDeviceCommand struct {
		ID      model.DeviceID
		Updates map[string]any
	}

	PatchDeviceCommandHandler = decorator.CommandHandler[PatchDeviceCommand, *model.Device]

	patchDeviceCommandHandler struct {
		deviceService ports.DevicesService
	}
)

func NewPatchDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) PatchDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[PatchDeviceCommand, *model.Device](
		patchDeviceCommandHandler{deviceService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h patchDeviceCommandHandler) Handle(ctx context.Context, cmd PatchDeviceCommand) (*model.Device, error) {
	return h.deviceService.PatchDevice(ctx, cmd.ID, cmd.Updates)
}
