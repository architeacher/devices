package runtime

import (
	"context"
	"fmt"
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/throttled/throttled/v2"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	infrastructureDep struct {
		publicHttpServer *http.Server
		adminHttpServer  *http.Server
		cacheClient      *infrastructure.KeydbClient
		logger           logger.Logger
		metricsClient    metrics.Client
		tracerProvider   otelTrace.TracerProvider
	}

	repositories struct {
		secretsRepo     ports.SecretsRepository
		idempotencyRepo ports.IdempotencyCache
		devicesCache    ports.DevicesCache
		rateLimitStore  throttled.GCRAStoreCtx
	}

	servicesDep struct {
		devices       ports.DevicesService
		healthChecker ports.HealthChecker
	}

	applications struct {
		webApp *usecases.WebApplication
	}

	dependencies struct {
		config       *config.ServiceConfig
		configLoader *config.Loader

		infra infrastructureDep

		repos repositories

		services servicesDep

		apps applications

		cleanupFuncs map[string]func(ctx context.Context) error
	}

	DependencyOption func(*dependencies) error
)

func initializeDependencies(ctx context.Context, opts ...DependencyOption) (*dependencies, error) {
	deps := &dependencies{
		cleanupFuncs: make(map[string]func(ctx context.Context) error),
	}

	allOpts := append(defaultOptions(ctx), opts...)

	for _, opt := range allOpts {
		if err := opt(deps); err != nil {
			return nil, fmt.Errorf("failed to apply dependency option: %w", err)
		}
	}

	return deps, nil
}
