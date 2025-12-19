package queries

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
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) GetDeviceQueryHandler {
	return decorator.ApplyQueryDecorators[GetDeviceQuery, *model.Device](
		getDeviceQueryHandler{deviceService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h getDeviceQueryHandler) Execute(ctx context.Context, query GetDeviceQuery) (*model.Device, error) {
	return h.deviceService.GetDevice(ctx, query.ID)
}
