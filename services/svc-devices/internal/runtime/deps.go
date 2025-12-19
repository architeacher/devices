package runtime

import (
	"context"
	"fmt"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/config"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/jackc/pgx/v5/pgxpool"
	otelTrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

type (
	infrastructureDep struct {
		grpcServer     *grpc.Server
		tracerProvider otelTrace.TracerProvider
		tracerShutdown func(ctx context.Context) error
		metricsClient  metrics.Client
		logger         logger.Logger
		dbPool         *pgxpool.Pool
	}

	repositories struct {
		deviceRepo ports.DeviceRepository
	}

	dependencies struct {
		config         *config.ServiceConfig
		infra          infrastructureDep
		repos          repositories
		devicesService ports.DevicesService
		app            *usecases.Application
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

func (d *dependencies) getDBHealthChecker() ports.DatabaseHealthChecker {
	return d.repos.deviceRepo.(*repos.DevicesRepository)
}
