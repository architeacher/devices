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
	CreateDeviceCommand struct {
		Name  string
		Brand string
		State model.State
	}

	CreateDeviceCommandHandler = decorator.CommandHandler[CreateDeviceCommand, *model.Device]

	createDeviceCommandHandler struct {
		devicesService ports.DevicesService
		cache          ports.DevicesCache
	}
)

func NewCreateDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) CreateDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[CreateDeviceCommand, *model.Device](
		createDeviceCommandHandler{devicesService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

// NewCreateDeviceCommandHandlerWithCache creates a command handler with cache invalidation.
func NewCreateDeviceCommandHandlerWithCache(
	svc ports.DevicesService,
	cache ports.DevicesCache,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) CreateDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[CreateDeviceCommand, *model.Device](
		createDeviceCommandHandler{devicesService: svc, cache: cache},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h createDeviceCommandHandler) Handle(ctx context.Context, cmd CreateDeviceCommand) (*model.Device, error) {
	device, err := h.devicesService.CreateDevice(ctx, cmd.Name, cmd.Brand, cmd.State)
	if err != nil {
		return nil, err
	}

	if h.cache != nil {
		go func() {
			_ = h.cache.InvalidateAllLists(context.Background())
		}()
	}

	return device, nil
}
