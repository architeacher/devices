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
	UpdateDeviceCommand struct {
		ID    model.DeviceID
		Name  string
		Brand string
		State model.State
	}

	UpdateDeviceCommandHandler = decorator.CommandHandler[UpdateDeviceCommand, *model.Device]

	updateDeviceCommandHandler struct {
		devicesService ports.DevicesService
	}
)

func NewUpdateDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) UpdateDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[UpdateDeviceCommand, *model.Device](
		updateDeviceCommandHandler{devicesService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h updateDeviceCommandHandler) Handle(ctx context.Context, cmd UpdateDeviceCommand) (*model.Device, error) {
	return h.devicesService.UpdateDevice(ctx, cmd.ID, cmd.Name, cmd.Brand, cmd.State)
}
