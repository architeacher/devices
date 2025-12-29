package runtime

import (
	"context"
	"fmt"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/services"
	"github.com/architeacher/devices/services/svc-devices/internal/config"
	"github.com/architeacher/devices/services/svc-devices/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/hashicorp/vault/api"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithConfig(),
		WithConfigLoader(ctx),
		WithSecretsRepository(),
		WithLogger(),
		WithDatabase(ctx),
		WithDataRepositories(),
		WithServices(),
		WithApplication(),
		WithGRPCServer(),
		WithMetrics(),
		WithTracing(),
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

func WithConfigLoader(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		if !d.config.SecretsStorage.Enabled || d.repos.secretsRepo == nil {
			return nil
		}

		loader := config.NewLoader(d.config, d.repos.secretsRepo, 0)

		version, err := loader.Load(ctx, d.repos.secretsRepo, d.config)
		if err != nil {
			return fmt.Errorf("loading secrets from Vault: %w", err)
		}

		d.configLoader = config.NewLoader(d.config, d.repos.secretsRepo, version)

		return nil
	}
}

func WithSecretsRepository() DependencyOption {
	return func(d *dependencies) error {
		if !d.config.SecretsStorage.Enabled {
			return nil
		}

		cfg := d.config.SecretsStorage

		vaultConfig := api.DefaultConfig()
		vaultConfig.Address = cfg.Address
		vaultConfig.Timeout = cfg.Timeout

		if cfg.TLSSkipVerify {
			tlsConfig := &api.TLSConfig{
				Insecure: true,
			}
			if err := vaultConfig.ConfigureTLS(tlsConfig); err != nil {
				return fmt.Errorf("failed to configure TLS: %w", err)
			}
		}

		client, err := api.NewClient(vaultConfig)
		if err != nil {
			return fmt.Errorf("creating Vault client: %w", err)
		}

		if cfg.Namespace != "" {
			client.SetNamespace(cfg.Namespace)
		}

		d.repos.secretsRepo = repos.NewVaultRepository(client)

		return nil
	}
}

func WithDatabase(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		pool, err := infrastructure.NewPool(ctx, d.config.Database)
		if err != nil {
			return fmt.Errorf("connecting to database: %w", err)
		}

		d.infra.dbPool = pool

		d.cleanupFuncs["DB server"] = func(ctx context.Context) error {
			d.infra.dbPool.Close()

			return nil
		}

		return nil
	}
}

func WithDataRepositories() DependencyOption {
	return func(d *dependencies) error {
		d.repos.deviceRepo = repos.NewDevicesRepository(
			d.infra.dbPool,
			repos.NewPgxScanner(),
			repos.NewCriteriaTranslator(&d.infra.logger),
			d.infra.logger,
		)

		return nil
	}
}

func WithServices() DependencyOption {
	return func(d *dependencies) error {
		d.services = servicesDep{
			devices: services.NewDevicesService(d.repos.deviceRepo),
		}

		return nil
	}
}

func WithApplication() DependencyOption {
	return func(d *dependencies) error {
		grpcApp := usecases.NewApplication(
			d.services.devices,
			d.getDBHealthChecker(),
			d.infra.logger,
			d.infra.tracerProvider,
			d.infra.metricsClient,
		)

		d.apps = applications{
			grpcApp: grpcApp,
		}

		return nil
	}
}

func WithGRPCServer() DependencyOption {
	return func(d *dependencies) error {
		opts := []grpc.ServerOption{
			grpc.MaxRecvMsgSize(d.config.GRPCServer.MaxRecvMsgSize),
			grpc.MaxSendMsgSize(d.config.GRPCServer.MaxSendMsgSize),
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
			grpc.ChainUnaryInterceptor(
				inboundgrpc.ContextExtractorInterceptor(),
				inboundgrpc.AccessLogInterceptor(d.infra.logger, d.config.Logging.AccessLog),
			),
		}

		server := grpc.NewServer(opts...)

		deviceHandler := inboundgrpc.NewDevicesHandler(d.apps.grpcApp)
		devicev1.RegisterDeviceServiceServer(server, deviceHandler)

		healthHandler := inboundgrpc.NewHealthHandler(d.getDBHealthChecker())
		devicev1.RegisterHealthServiceServer(server, healthHandler)

		reflection.Register(server)

		d.infra.grpcServer = server

		d.cleanupFuncs["GRPC server"] = func(ctx context.Context) error {
			d.infra.grpcServer.GracefulStop()

			return nil
		}

		return nil
	}
}

func WithLogger() DependencyOption {
	return func(d *dependencies) error {
		d.infra.logger = logger.New(d.config.Logging.Level, d.config.Logging.Format)

		return nil
	}
}

func WithMetrics() DependencyOption {
	return func(d *dependencies) error {
		d.infra.metricsClient = noop.NewMetricsClient()

		return nil
	}
}

func WithTracing() DependencyOption {
	return func(d *dependencies) error {
		if d.config.Telemetry.OTLPEndpoint == "" {
			d.infra.tracerProvider = infrastructure.NewNoopTracerProvider()

			return nil
		}

		tp, shutdown, err := infrastructure.NewTracerProvider(
			d.config.Telemetry.ServiceName,
			d.config.Telemetry.ServiceVersion,
			d.config.Telemetry.OTLPEndpoint,
		)
		if err != nil {
			return fmt.Errorf("initializing tracer: %w", err)
		}

		d.infra.tracerProvider = tp

		d.cleanupFuncs["tracer"] = shutdown

		return nil
	}
}
