package runtime

import (
	"context"
	"fmt"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/config"
	infraPostgres "github.com/architeacher/devices/services/svc-devices/internal/infrastructure/postgres"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-devices/internal/services"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithConfig(),
		WithLogger(),
		WithTracing(ctx),
		WithMetrics(ctx),
		WithDatabase(ctx),
		WithDevicesRepository(),
		WithDevicesService(),
		WithApplication(),
		WithGRPCServer(),
	}
}

func WithConfig() DependencyOption {
	return func(d *dependencies) error {
		cfg, err := config.Init()
		if err != nil {
			return fmt.Errorf("initializing configuration: %w", err)
		}

		d.config = cfg

		return nil
	}
}

func WithLogger() DependencyOption {
	return func(d *dependencies) error {
		d.infra.logger = logger.New(d.config.Logging.Level, d.config.Logging.Format)

		return nil
	}
}

func WithTracing(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		if d.config.Telemetry.OTLPEndpoint == "" {
			d.infra.tracerProvider = telemetry.NewNoopTracerProvider()

			return nil
		}

		tp, shutdown, err := telemetry.NewTracerProvider(
			d.config.Telemetry.ServiceName,
			d.config.Telemetry.ServiceVersion,
			d.config.Telemetry.OTLPEndpoint,
		)
		if err != nil {
			return fmt.Errorf("initializing tracer: %w", err)
		}

		d.infra.tracerProvider = tp
		d.infra.tracerShutdown = shutdown

		return nil
	}
}

func WithMetrics(_ context.Context) DependencyOption {
	return func(d *dependencies) error {
		d.infra.metricsClient = noop.NewMetricsClient()

		return nil
	}
}

func WithDatabase(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		pool, err := infraPostgres.NewPool(ctx, d.config.Database)
		if err != nil {
			return fmt.Errorf("connecting to database: %w", err)
		}

		d.infra.dbPool = pool

		return nil
	}
}

func WithDevicesRepository() DependencyOption {
	return func(d *dependencies) error {
		d.repos.deviceRepo = repos.NewDevicesRepository(d.infra.dbPool)

		return nil
	}
}

func WithDevicesService() DependencyOption {
	return func(d *dependencies) error {
		d.devicesService = services.NewDevicesService(d.repos.deviceRepo)

		return nil
	}
}

func WithApplication() DependencyOption {
	return func(d *dependencies) error {
		d.app = usecases.NewApplication(
			d.devicesService,
			d.getDBHealthChecker(),
			d.infra.logger,
			d.infra.tracerProvider,
			d.infra.metricsClient,
		)

		return nil
	}
}

func WithGRPCServer() DependencyOption {
	return func(d *dependencies) error {
		opts := []grpc.ServerOption{
			grpc.MaxRecvMsgSize(d.config.GRPCServer.MaxRecvMsgSize),
			grpc.MaxSendMsgSize(d.config.GRPCServer.MaxSendMsgSize),
		}

		server := grpc.NewServer(opts...)

		deviceHandler := inboundgrpc.NewDevicesHandler(d.app)
		devicev1.RegisterDeviceServiceServer(server, deviceHandler)

		healthHandler := inboundgrpc.NewHealthHandler(d.getDBHealthChecker())
		devicev1.RegisterHealthServiceServer(server, healthHandler)

		reflection.Register(server)

		d.infra.grpcServer = server

		return nil
	}
}
