package queries

import (
	"context"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/shared/decorator"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	FetchLivenessQuery struct{}

	FetchLivenessQueryHandler = decorator.QueryHandler[FetchLivenessQuery, *model.LivenessReport]

	fetchLivenessQueryHandler struct {
		healthChecker ports.HealthChecker
	}
)

func NewFetchLivenessQueryHandler(
	healthChecker ports.HealthChecker,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) FetchLivenessQueryHandler {
	return decorator.ApplyQueryDecorators[FetchLivenessQuery, *model.LivenessReport](
		fetchLivenessQueryHandler{healthChecker: healthChecker},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchLivenessQueryHandler) Execute(ctx context.Context, _ FetchLivenessQuery) (*model.LivenessReport, error) {
	return h.healthChecker.Liveness(ctx)
}
