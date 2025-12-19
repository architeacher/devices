package runtime

import (
	"context"
	"fmt"
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	tracerShutdownFunc func(ctx context.Context) error

	infrastructureDep struct {
		httpServer     *http.Server
		tracerProvider otelTrace.TracerProvider
		tracerShutdown tracerShutdownFunc
		metricsClient  metrics.Client
		logger         logger.Logger
	}

	repositories struct {
		secretsRepo ports.SecretsRepository
	}

	dependencies struct {
		config         *config.ServiceConfig
		configLoader   *config.Loader
		infra          infrastructureDep
		repos          repositories
		app            *usecases.WebApplication
		devicesService ports.DevicesService
		healthChecker  ports.HealthChecker
	}

	DependencyOption func(*dependencies) error
)

func initializeDependencies(ctx context.Context, opts ...DependencyOption) (*dependencies, error) {
	deps := &dependencies{}

	allOpts := append(defaultOptions(ctx), opts...)

	for _, opt := range allOpts {
		if err := opt(deps); err != nil {
			return nil, fmt.Errorf("failed to apply dependency option: %w", err)
		}
	}

	return deps, nil
}
