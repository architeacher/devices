package usecases

import (
	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/commands"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	// CacheOptions holds cache configuration for the web application.
	CacheOptions struct {
		Cache            ports.DevicesCache
		GetDeviceConfig  decorator.CacheConfig
		ListDeviceConfig decorator.CacheConfig
	}

	Commands struct {
		CreateDevice commands.CreateDeviceCommandHandler
		UpdateDevice commands.UpdateDeviceCommandHandler
		PatchDevice  commands.PatchDeviceCommandHandler
		DeleteDevice commands.DeleteDeviceCommandHandler
	}

	Queries struct {
		GetDevice         queries.GetDeviceQueryHandler
		ListDevices       queries.ListDevicesQueryHandler
		FetchLiveness     queries.FetchLivenessQueryHandler
		FetchReadiness    queries.FetchReadinessQueryHandler
		FetchHealthReport queries.FetchHealthReportQueryHandler
	}

	WebApplication struct {
		Commands Commands
		Queries  Queries
	}
)

func NewWebApplication(
	deviceSvc ports.DevicesService,
	healthChecker ports.HealthChecker,
	cacheOpts *CacheOptions,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) *WebApplication {
	return &WebApplication{
		Commands: buildCommands(deviceSvc, cacheOpts, log, metricsClient, tracerProvider),
		Queries:  buildQueries(deviceSvc, healthChecker, cacheOpts, log, metricsClient, tracerProvider),
	}
}

func buildCommands(
	deviceSvc ports.DevicesService,
	cacheOpts *CacheOptions,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) Commands {
	if cacheOpts != nil && cacheOpts.Cache != nil {
		return Commands{
			CreateDevice: commands.NewCreateDeviceCommandHandlerWithCache(deviceSvc, cacheOpts.Cache, log, metricsClient, tracerProvider),
			UpdateDevice: commands.NewUpdateDeviceCommandHandlerWithCache(deviceSvc, cacheOpts.Cache, log, metricsClient, tracerProvider),
			PatchDevice:  commands.NewPatchDeviceCommandHandlerWithCache(deviceSvc, cacheOpts.Cache, log, metricsClient, tracerProvider),
			DeleteDevice: commands.NewDeleteDeviceCommandHandlerWithCache(deviceSvc, cacheOpts.Cache, log, metricsClient, tracerProvider),
		}
	}

	return Commands{
		CreateDevice: commands.NewCreateDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
		UpdateDevice: commands.NewUpdateDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
		PatchDevice:  commands.NewPatchDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
		DeleteDevice: commands.NewDeleteDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
	}
}

func buildQueries(
	deviceSvc ports.DevicesService,
	healthChecker ports.HealthChecker,
	cacheOpts *CacheOptions,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) Queries {
	q := Queries{
		FetchLiveness:     queries.NewFetchLivenessQueryHandler(healthChecker, log, metricsClient, tracerProvider),
		FetchReadiness:    queries.NewFetchReadinessQueryHandler(healthChecker, log, metricsClient, tracerProvider),
		FetchHealthReport: queries.NewFetchHealthReportQueryHandler(healthChecker, log, metricsClient, tracerProvider),
	}

	if cacheOpts != nil && cacheOpts.Cache != nil {
		q.GetDevice = queries.NewGetDeviceQueryHandlerWithCache(
			deviceSvc,
			repos.NewGetDeviceCacheAdapter(cacheOpts.Cache),
			cacheOpts.GetDeviceConfig,
			log,
			metricsClient,
			tracerProvider,
		)
		q.ListDevices = queries.NewListDevicesQueryHandlerWithCache(
			deviceSvc,
			repos.NewListDevicesCacheAdapter(cacheOpts.Cache),
			cacheOpts.ListDeviceConfig,
			log,
			metricsClient,
			tracerProvider,
		)
	} else {
		q.GetDevice = queries.NewGetDeviceQueryHandler(deviceSvc, log, metricsClient, tracerProvider)
		q.ListDevices = queries.NewListDevicesQueryHandler(deviceSvc, log, metricsClient, tracerProvider)
	}

	return q
}
