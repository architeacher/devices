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
	CreateDeviceCommand struct {
		Name  string
		Brand string
		State model.State
	}

	CreateDeviceCommandHandler = decorator.CommandHandler[CreateDeviceCommand, *model.Device]

	createDeviceCommandHandler struct {
		devicesService ports.DevicesService
	}
)

func NewCreateDeviceCommandHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) CreateDeviceCommandHandler {
	return decorator.ApplyCommandDecorators[CreateDeviceCommand, *model.Device](
		createDeviceCommandHandler{devicesService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h createDeviceCommandHandler) Handle(ctx context.Context, cmd CreateDeviceCommand) (*model.Device, error) {
	return h.devicesService.CreateDevice(ctx, cmd.Name, cmd.Brand, cmd.State)
}
