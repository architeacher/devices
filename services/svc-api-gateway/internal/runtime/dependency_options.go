package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	inboundhttp "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/services"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/hashicorp/vault/api"
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithConfig(),
		WithConfigLoader(ctx),
		WithSecretsRepository(),
		WithLogger(),
		WithCache(ctx),
		WithDataRepositories(),
		WithServices(),
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

func WithCache(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		cacheClient := infrastructure.NewKeyDBClient(d.config.Cache, d.infra.logger)

		cacheCtx, cancel := context.WithTimeout(ctx, d.config.Cache.DialTimeout)
		defer cancel()

		if err := cacheClient.Ping(cacheCtx); err != nil {
			d.infra.logger.Error().Err(err).Msg("failed to connect to cache, continuing without cache")
			d.infra.cacheClient = nil

			return fmt.Errorf("pinging cache: %w", err)
		}

		d.infra.cacheClient = cacheClient

		d.cleanupFuncs["cache"] = func(ctx context.Context) error {
			return d.infra.cacheClient.Close()
		}

		d.infra.logger.Info().Msg("cache connection established")

		return nil
	}
}

func WithDataRepositories() DependencyOption {
	return func(d *dependencies) error {
		if d.config.Idempotency.Enabled && d.infra.cacheClient != nil {
			repo, err := repos.NewIdempotencyRepository(d.infra.cacheClient)
			if err != nil {
				return fmt.Errorf("creating idempotency repository: %w", err)
			}

			d.repos.idempotencyRepo = repo

			d.cleanupFuncs["idempotency repository"] = func(ctx context.Context) error {
				return repo.Close()
			}
		}

		if d.config.ThrottledRateLimiting.Enabled && d.infra.cacheClient != nil {
			store, err := repos.NewRateLimitStore(d.infra.cacheClient)
			if err != nil {
				return fmt.Errorf("creating rate limit store: %w", err)
			}

			d.repos.rateLimitStore = store
		}

		return nil
	}
}

func WithServices() DependencyOption {
	return func(d *dependencies) error {
		client, err := services.NewDevicesService(d.config)
		if err != nil {
			return fmt.Errorf("creating devices gRPC client: %w", err)
		}

		d.services = servicesDep{
			devices:       client,
			healthChecker: client,
		}

		d.cleanupFuncs["GRPC client"] = func(ctx context.Context) error {
			return client.Close()
		}

		return nil
	}
}

func WithApplication() DependencyOption {
	return func(d *dependencies) error {
		webApp := usecases.NewWebApplication(
			d.services.devices,
			d.services.healthChecker,
			d.infra.logger,
			d.infra.metricsClient,
			d.infra.tracerProvider,
		)

		d.apps = applications{
			webApp: webApp,
		}

		return nil
	}
}

func WithHTTPServer() DependencyOption {
	return func(d *dependencies) error {
		cfg := d.config.HTTPServer

		router := inboundhttp.NewRouter(inboundhttp.RouterConfig{
			App:             d.apps.webApp,
			IdempotencyRepo: d.repos.idempotencyRepo,
			RateLimitStore:  d.repos.rateLimitStore,
			ServiceConfig:   d.config,
			Logger:          d.infra.logger,
			MetricsClient:   d.infra.metricsClient,
		})

		d.infra.logger.Info().Msg("creating HTTP server...")

		d.infra.httpServer = &http.Server{
			Addr:         net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		}

		d.cleanupFuncs["HTTP server"] = d.infra.httpServer.Shutdown

		d.infra.logger.Info().Str("addr", d.infra.httpServer.Addr).Msg("HTTP server created")

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
		if !d.config.Telemetry.Traces.Enabled || d.config.Telemetry.OTLPEndpoint == "" {
			d.infra.tracerProvider = infrastructure.NewNoopTracerProvider()

			return nil
		}

		tp, shutdown, err := infrastructure.NewTracerProvider(
			d.config.App,
			d.config.Telemetry,
		)
		if err != nil {
			return fmt.Errorf("initializing tracer: %w", err)
		}

		d.infra.tracerProvider = tp

		d.cleanupFuncs["tracing"] = shutdown

		return nil
	}
}
