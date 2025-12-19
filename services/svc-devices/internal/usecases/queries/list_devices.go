package queries

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
	ListDevicesQuery struct {
		Filter model.DeviceFilter
	}

	ListDevicesQueryHandler = decorator.QueryHandler[ListDevicesQuery, *model.DeviceList]

	listDevicesQueryHandler struct {
		devicesService ports.DevicesService
	}
)

func NewListDevicesQueryHandler(
	svc ports.DevicesService,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) ListDevicesQueryHandler {
	return decorator.ApplyQueryDecorators[ListDevicesQuery, *model.DeviceList](
		listDevicesQueryHandler{devicesService: svc},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h listDevicesQueryHandler) Execute(ctx context.Context, query ListDevicesQuery) (*model.DeviceList, error) {
	return h.devicesService.ListDevices(ctx, query.Filter)
}
