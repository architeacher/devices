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
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) *WebApplication {
	return &WebApplication{
		Commands: Commands{
			CreateDevice: commands.NewCreateDeviceCommandHandler(deviceSvc, log, tracerProvider, metricsClient),
			UpdateDevice: commands.NewUpdateDeviceCommandHandler(deviceSvc, log, tracerProvider, metricsClient),
			PatchDevice:  commands.NewPatchDeviceCommandHandler(deviceSvc, log, tracerProvider, metricsClient),
			DeleteDevice: commands.NewDeleteDeviceCommandHandler(deviceSvc, log, tracerProvider, metricsClient),
		},
		Queries: Queries{
			GetDevice:         queries.NewGetDeviceQueryHandler(deviceSvc, log, tracerProvider, metricsClient),
			ListDevices:       queries.NewListDevicesQueryHandler(deviceSvc, log, tracerProvider, metricsClient),
			FetchLiveness:     queries.NewFetchLivenessQueryHandler(healthChecker, log, tracerProvider, metricsClient),
			FetchReadiness:    queries.NewFetchReadinessQueryHandler(healthChecker, log, tracerProvider, metricsClient),
			FetchHealthReport: queries.NewFetchHealthReportQueryHandler(healthChecker, log, tracerProvider, metricsClient),
		},
	}
}
