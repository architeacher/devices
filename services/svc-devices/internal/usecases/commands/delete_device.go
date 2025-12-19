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
	DeleteDeviceCommand struct {
		ID model.DeviceID
	}

	DeleteDeviceCommandHandler = decorator.CommandHandler[DeleteDeviceCommand, struct{}]

	deleteDeviceCommandHandler struct {
		devicesService ports.DevicesService
	}
)

func NewDeleteDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) DeleteDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[DeleteDeviceCommand, struct{}](
		deleteDeviceCommandHandler{devicesService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h deleteDeviceCommandHandler) Handle(ctx context.Context, cmd DeleteDeviceCommand) (struct{}, error) {
	if err := h.devicesService.DeleteDevice(ctx, cmd.ID); err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
}
