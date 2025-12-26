package queries

import (
	"context"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	FetchReadinessQuery struct{}

	FetchReadinessQueryHandler = decorator.QueryHandler[FetchReadinessQuery, *model.ReadinessReport]

	fetchReadinessQueryHandler struct {
		healthChecker ports.HealthChecker
	}
)

func NewFetchReadinessQueryHandler(
	healthChecker ports.HealthChecker,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) FetchReadinessQueryHandler {
	return decorator.ApplyQueryDecorators[FetchReadinessQuery, *model.ReadinessReport](
		fetchReadinessQueryHandler{healthChecker: healthChecker},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h fetchReadinessQueryHandler) Execute(ctx context.Context, _ FetchReadinessQuery) (*model.ReadinessReport, error) {
	return h.healthChecker.Readiness(ctx)
}
