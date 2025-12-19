package commands

import (
	"context"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
	"github.com/architeacher/devices/services/svc-devices/shared/decorator"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	PatchDeviceCommand struct {
		ID      model.DeviceID
		Updates map[string]any
	}

	PatchDeviceCommandHandler = decorator.CommandHandler[PatchDeviceCommand, *model.Device]

	patchDeviceCommandHandler struct {
		devicesService ports.DevicesService
	}
)

func NewPatchDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) PatchDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[PatchDeviceCommand, *model.Device](
		patchDeviceCommandHandler{devicesService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h patchDeviceCommandHandler) Handle(ctx context.Context, cmd PatchDeviceCommand) (*model.Device, error) {
	return h.devicesService.PatchDevice(ctx, cmd.ID, cmd.Updates)
}
