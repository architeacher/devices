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
	DeleteDeviceCommand struct {
		ID model.DeviceID
	}

	DeleteDeviceResult struct {
		Success bool
	}

	DeleteDeviceCommandHandler = decorator.CommandHandler[DeleteDeviceCommand, DeleteDeviceResult]

	deleteDeviceCommandHandler struct {
		deviceService ports.DevicesService
	}
)

func NewDeleteDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) DeleteDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[DeleteDeviceCommand, DeleteDeviceResult](
		deleteDeviceCommandHandler{deviceService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h deleteDeviceCommandHandler) Handle(ctx context.Context, cmd DeleteDeviceCommand) (DeleteDeviceResult, error) {
	if err := h.deviceService.DeleteDevice(ctx, cmd.ID); err != nil {
		return DeleteDeviceResult{Success: false}, err
	}

	return DeleteDeviceResult{Success: true}, nil
}
