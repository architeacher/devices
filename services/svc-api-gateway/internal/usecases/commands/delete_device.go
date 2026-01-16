package commands

import (
	"context"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
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
		cache         ports.DevicesCache
	}
)

func NewDeleteDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) DeleteDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[DeleteDeviceCommand, DeleteDeviceResult](
		deleteDeviceCommandHandler{deviceService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

// NewDeleteDeviceCommandHandlerWithCache creates a command handler with cache invalidation.
func NewDeleteDeviceCommandHandlerWithCache(
	svc ports.DevicesService,
	cache ports.DevicesCache,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) DeleteDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[DeleteDeviceCommand, DeleteDeviceResult](
		deleteDeviceCommandHandler{deviceService: svc, cache: cache},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h deleteDeviceCommandHandler) Handle(ctx context.Context, cmd DeleteDeviceCommand) (DeleteDeviceResult, error) {
	if err := h.deviceService.DeleteDevice(ctx, cmd.ID); err != nil {
		return DeleteDeviceResult{Success: false}, err
	}

	if h.cache != nil {
		go func() {
			bgCtx := context.Background()
			_ = h.cache.InvalidateDevice(bgCtx, cmd.ID)
			_ = h.cache.InvalidateAllLists(bgCtx)
		}()
	}

	return DeleteDeviceResult{Success: true}, nil
}
