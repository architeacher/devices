package usecases

import (
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/commands"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
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
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) *WebApplication {
	return &WebApplication{
		Commands: Commands{
			CreateDevice: commands.NewCreateDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
			UpdateDevice: commands.NewUpdateDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
			PatchDevice:  commands.NewPatchDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
			DeleteDevice: commands.NewDeleteDeviceCommandHandler(deviceSvc, log, metricsClient, tracerProvider),
		},
		Queries: Queries{
			GetDevice:         queries.NewGetDeviceQueryHandler(deviceSvc, log, metricsClient, tracerProvider),
			ListDevices:       queries.NewListDevicesQueryHandler(deviceSvc, log, metricsClient, tracerProvider),
			FetchLiveness:     queries.NewFetchLivenessQueryHandler(healthChecker, log, metricsClient, tracerProvider),
			FetchReadiness:    queries.NewFetchReadinessQueryHandler(healthChecker, log, metricsClient, tracerProvider),
			FetchHealthReport: queries.NewFetchHealthReportQueryHandler(healthChecker, log, metricsClient, tracerProvider),
		},
	}
}
