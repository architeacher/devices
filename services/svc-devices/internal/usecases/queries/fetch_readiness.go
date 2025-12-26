package queries

import (
	"context"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	FetchReadinessQuery struct{}

	ReadinessResult struct {
		Status string `json:"status"`
		Ready  bool   `json:"ready"`
	}

	FetchReadinessQueryHandler = decorator.QueryHandler[FetchReadinessQuery, *ReadinessResult]

	fetchReadinessQueryHandler struct {
		dbHealthChecker ports.DatabaseHealthChecker
	}
)

func NewFetchReadinessQueryHandler(
	dbHealthChecker ports.DatabaseHealthChecker,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) FetchReadinessQueryHandler {
	return decorator.ApplyQueryDecorators[FetchReadinessQuery, *ReadinessResult](
		fetchReadinessQueryHandler{dbHealthChecker: dbHealthChecker},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h fetchReadinessQueryHandler) Execute(ctx context.Context, _ FetchReadinessQuery) (*ReadinessResult, error) {
	if err := h.dbHealthChecker.Ping(ctx); err != nil {
		return &ReadinessResult{
			Status: "unavailable",
			Ready:  false,
		}, nil
	}

	return &ReadinessResult{
		Status: "ok",
		Ready:  true,
	}, nil
}
