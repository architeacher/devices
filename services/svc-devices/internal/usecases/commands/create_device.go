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

func (h createDeviceCommandHandler) Handle(ctx context.Context, cmd CreateDeviceCommand) (*model.Device, error) {
	return h.devicesService.CreateDevice(ctx, cmd.Name, cmd.Brand, cmd.State)
}
