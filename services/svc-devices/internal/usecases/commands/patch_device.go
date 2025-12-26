package commands

import (
	"context"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
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
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) PatchDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[PatchDeviceCommand, *model.Device](
		patchDeviceCommandHandler{devicesService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h patchDeviceCommandHandler) Handle(ctx context.Context, cmd PatchDeviceCommand) (*model.Device, error) {
	return h.devicesService.PatchDevice(ctx, cmd.ID, cmd.Updates)
}
