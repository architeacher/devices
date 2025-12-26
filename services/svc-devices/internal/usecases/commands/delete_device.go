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
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) DeleteDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[DeleteDeviceCommand, struct{}](
		deleteDeviceCommandHandler{devicesService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h deleteDeviceCommandHandler) Handle(ctx context.Context, cmd DeleteDeviceCommand) (struct{}, error) {
	if err := h.devicesService.DeleteDevice(ctx, cmd.ID); err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
}
