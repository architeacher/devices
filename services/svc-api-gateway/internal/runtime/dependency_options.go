package runtime

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	inboundhttp "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/outbound/devices"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure/telemetry"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/hashicorp/vault/api"
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithConfig(),
		WithConfigLoader(ctx),
		WithLogger(),
		WithSecretsRepository(),
		WithDeviceService(),
		WithApplication(),
		WithHTTPServer(),
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

func WithSecretsRepository() DependencyOption {
	return func(d *dependencies) error {
		if !d.config.SecretStorage.Enabled {
			return nil
		}

		vaultConfig := api.DefaultConfig()
		vaultConfig.Address = d.config.SecretStorage.Address
		vaultConfig.Timeout = d.config.SecretStorage.Timeout

		if d.config.SecretStorage.TLSSkipVerify {
			vaultConfig.HttpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}

		client, err := api.NewClient(vaultConfig)
		if err != nil {
			return fmt.Errorf("creating Vault client: %w", err)
		}

		if d.config.SecretStorage.Namespace != "" {
			client.SetNamespace(d.config.SecretStorage.Namespace)
		}

		d.repos.secretsRepo = repos.NewVaultRepository(client)

		return nil
	}
}

func WithConfigLoader(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		if !d.config.SecretStorage.Enabled || d.repos.secretsRepo == nil {
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

func WithLogger() DependencyOption {
	return func(d *dependencies) error {
		d.infra.logger = logger.New(d.config.Logging.Level, d.config.Logging.Format)

		return nil
	}
}

func WithDeviceService() DependencyOption {
	return func(d *dependencies) error {
		client, err := devices.NewClient(d.config.Devices, d.config.Backoff)
		if err != nil {
			return fmt.Errorf("creating devices gRPC client: %w", err)
		}

		d.devicesService = client
		d.healthChecker = client
		d.grpcCleanup = append(d.grpcCleanup, client.Close)

		return nil
	}
}

func WithApplication() DependencyOption {
	return func(d *dependencies) error {
		d.app = usecases.NewWebApplication(
			d.devicesService,
			d.healthChecker,
			d.infra.logger,
			d.infra.tracerProvider,
			d.infra.metricsClient,
		)

		return nil
	}
}

func WithHTTPServer() DependencyOption {
	return func(d *dependencies) error {
		router := inboundhttp.NewRouter(inboundhttp.RouterConfig{
			App:           d.app,
			Logger:        d.infra.logger,
			MetricsClient: d.infra.metricsClient,
			Config:        d.config,
		})

		d.infra.httpServer = &http.Server{
			Handler:      router,
			ReadTimeout:  d.config.HTTPServer.ReadTimeout,
			WriteTimeout: d.config.HTTPServer.WriteTimeout,
			IdleTimeout:  d.config.HTTPServer.IdleTimeout,
		}

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
