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
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) UpdateDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[UpdateDeviceCommand, *model.Device](
		updateDeviceCommandHandler{devicesService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h updateDeviceCommandHandler) Handle(ctx context.Context, cmd UpdateDeviceCommand) (*model.Device, error) {
	return h.devicesService.UpdateDevice(ctx, cmd.ID, cmd.Name, cmd.Brand, cmd.State)
}
