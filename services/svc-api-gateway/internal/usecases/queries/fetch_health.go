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
	FetchHealthReportQuery struct{}

	FetchHealthReportQueryHandler = decorator.QueryHandler[FetchHealthReportQuery, *model.HealthReport]

	fetchHealthReportQueryHandler struct {
		healthChecker ports.HealthChecker
	}
)

func NewFetchHealthReportQueryHandler(
	healthChecker ports.HealthChecker,
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) FetchHealthReportQueryHandler {
	return decorator.ApplyQueryDecorators[FetchHealthReportQuery, *model.HealthReport](
		fetchHealthReportQueryHandler{healthChecker: healthChecker},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchHealthReportQueryHandler) Execute(ctx context.Context, _ FetchHealthReportQuery) (*model.HealthReport, error) {
	return h.healthChecker.Health(ctx)
}
