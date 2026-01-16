package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	inboundhttp "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http"
	grpcclient "github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/outbound/grpc"
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
		WithPublicHTTPServer(),
		WithAdminHTTPServer(),
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

		if d.config.DevicesCache.Enabled && d.infra.cacheClient != nil {
			d.repos.devicesCache = repos.NewDevicesCacheRepository(d.infra.cacheClient, d.infra.logger)
			d.infra.logger.Info().Msg("devices cache repository initialized")
		}

		return nil
	}
}

func WithServices() DependencyOption {
	return func(d *dependencies) error {
		conn, err := infrastructure.NewGRPCConnection(d.config)
		if err != nil {
			return fmt.Errorf("creating gRPC connection: %w", err)
		}

		client := grpcclient.NewClient(conn, d.config)
		svc := services.NewDevicesService(client)

		d.services = servicesDep{
			devices:       svc,
			healthChecker: svc,
		}

		d.cleanupFuncs["gRPC connection"] = func(ctx context.Context) error {
			return conn.Close()
		}

		return nil
	}
}

func WithApplication() DependencyOption {
	return func(d *dependencies) error {
		var cacheOpts *usecases.CacheOptions

		if d.repos.devicesCache != nil {
			cacheOpts = &usecases.CacheOptions{
				Cache: d.repos.devicesCache,
				GetDeviceConfig: decorator.CacheConfig{
					Enabled: d.config.DevicesCache.Enabled,
					TTL:     d.config.DevicesCache.DeviceTTL,
				},
				ListDeviceConfig: decorator.CacheConfig{
					Enabled: d.config.DevicesCache.Enabled,
					TTL:     d.config.DevicesCache.ListTTL,
				},
			}
		}

		webApp := usecases.NewWebApplication(
			d.services.devices,
			d.services.healthChecker,
			cacheOpts,
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

func WithPublicHTTPServer() DependencyOption {
	return func(d *dependencies) error {
		cfg := d.config.PublicHTTPServer

		router := inboundhttp.NewRouter(inboundhttp.RouterConfig{
			App:             d.apps.webApp,
			IdempotencyRepo: d.repos.idempotencyRepo,
			RateLimitStore:  d.repos.rateLimitStore,
			ServiceConfig:   d.config,
			Logger:          d.infra.logger,
			MetricsClient:   d.infra.metricsClient,
		})

		d.infra.logger.Info().Msg("creating public HTTP server...")

		d.infra.publicHttpServer = &http.Server{
			Addr:         net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		}

		d.cleanupFuncs["public HTTP server"] = d.infra.publicHttpServer.Shutdown

		d.infra.logger.Info().Str("addr", d.infra.publicHttpServer.Addr).Msg("public HTTP server created")

		return nil
	}
}

func WithAdminHTTPServer() DependencyOption {
	return func(d *dependencies) error {
		cfg := d.config.AdminHTTPServer

		if !cfg.Enabled {
			d.infra.logger.Info().Msg("admin HTTP server disabled")

			return nil
		}

		router := inboundhttp.NewAdminRouter(inboundhttp.AdminRouterConfig{
			DevicesCache: d.repos.devicesCache,
			Logger:       d.infra.logger,
		})

		d.infra.logger.Info().Msg("creating admin HTTP server...")

		d.infra.adminHttpServer = &http.Server{
			Addr:         net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		}

		d.cleanupFuncs["admin HTTP server"] = d.infra.adminHttpServer.Shutdown

		d.infra.logger.Info().Str("addr", d.infra.adminHttpServer.Addr).Msg("admin HTTP server created")

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
