package queries

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
	GetDeviceQuery struct {
		ID model.DeviceID
	}

	GetDeviceQueryHandler = decorator.QueryHandler[GetDeviceQuery, *model.Device]

	getDeviceQueryHandler struct {
		deviceService ports.DevicesService
	}
)

func NewGetDeviceQueryHandler(
	svc ports.DevicesService,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) GetDeviceQueryHandler {
	return decorator.ApplyQueryDecorators[GetDeviceQuery, *model.Device](
		getDeviceQueryHandler{deviceService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h getDeviceQueryHandler) Execute(ctx context.Context, query GetDeviceQuery) (*model.Device, error) {
	return h.deviceService.GetDevice(ctx, query.ID)
}
