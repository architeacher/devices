package usecases

import (
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/commands"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases/queries"
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

	Application struct {
		Commands Commands
		Queries  Queries
	}
)

func NewApplication(
	devicesSvc ports.DevicesService,
	dbHealthChecker ports.DatabaseHealthChecker,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) *Application {
	return &Application{
		Commands: Commands{
			CreateDevice: commands.NewCreateDeviceCommandHandler(devicesSvc, log, tracerProvider, metricsClient),
			UpdateDevice: commands.NewUpdateDeviceCommandHandler(devicesSvc, log, tracerProvider, metricsClient),
			PatchDevice:  commands.NewPatchDeviceCommandHandler(devicesSvc, log, tracerProvider, metricsClient),
			DeleteDevice: commands.NewDeleteDeviceCommandHandler(devicesSvc, log, tracerProvider, metricsClient),
		},
		Queries: Queries{
			GetDevice:         queries.NewGetDeviceQueryHandler(devicesSvc, log, tracerProvider, metricsClient),
			ListDevices:       queries.NewListDevicesQueryHandler(devicesSvc, log, tracerProvider, metricsClient),
			FetchLiveness:     queries.NewFetchLivenessQueryHandler(log, tracerProvider, metricsClient),
			FetchReadiness:    queries.NewFetchReadinessQueryHandler(dbHealthChecker, log, tracerProvider, metricsClient),
			FetchHealthReport: queries.NewFetchHealthReportQueryHandler(dbHealthChecker, log, tracerProvider, metricsClient),
		},
	}
}
