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
	// ListDevicesCache is the cache interface for ListDevicesQuery.
	ListDevicesCache = decorator.Cache[ListDevicesQuery, *model.DeviceList]

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
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) ListDevicesQueryHandler {
	return decorator.ApplyQueryDecorators[ListDevicesQuery, *model.DeviceList](
		listDevicesQueryHandler{deviceService: svc},
		log,
		metricsClient,
		tracerProvider,
	)
}

// NewListDevicesQueryHandlerWithCache creates a query handler with caching support.
func NewListDevicesQueryHandlerWithCache(
	svc ports.DevicesService,
	cacheAdapter ListDevicesCache,
	cacheConfig decorator.CacheConfig,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) ListDevicesQueryHandler {
	return decorator.ApplyQueryDecoratorsWithCache[ListDevicesQuery, *model.DeviceList](
		listDevicesQueryHandler{deviceService: svc},
		cacheAdapter,
		cacheConfig,
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h listDevicesQueryHandler) Execute(ctx context.Context, query ListDevicesQuery) (*model.DeviceList, error) {
	return h.deviceService.ListDevices(ctx, query.Filter)
}
