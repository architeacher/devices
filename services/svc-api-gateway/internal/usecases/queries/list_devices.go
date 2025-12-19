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
	ListDevicesQuery struct {
		Filter model.DeviceFilter
	}

	ListDevicesQueryHandler = decorator.QueryHandler[ListDevicesQuery, *model.DeviceList]

	listDevicesQueryHandler struct {
		deviceService ports.DevicesService
	}
)

func NewListDevicesQueryHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) ListDevicesQueryHandler {
	return decorator.ApplyQueryDecorators[ListDevicesQuery, *model.DeviceList](
		listDevicesQueryHandler{deviceService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h listDevicesQueryHandler) Execute(ctx context.Context, query ListDevicesQuery) (*model.DeviceList, error) {
	return h.deviceService.ListDevices(ctx, query.Filter)
}
