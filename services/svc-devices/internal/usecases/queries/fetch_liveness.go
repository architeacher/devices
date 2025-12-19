package queries

import (
	"context"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/shared/decorator"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	FetchLivenessQuery struct{}

	LivenessResult struct {
		Status string `json:"status"`
	}

	FetchLivenessQueryHandler = decorator.QueryHandler[FetchLivenessQuery, *LivenessResult]

	fetchLivenessQueryHandler struct{}
)

func NewFetchLivenessQueryHandler(
	log logger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient metrics.Client,
) FetchLivenessQueryHandler {
	return decorator.ApplyQueryDecorators[FetchLivenessQuery, *LivenessResult](
		fetchLivenessQueryHandler{},
		log,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchLivenessQueryHandler) Execute(_ context.Context, _ FetchLivenessQuery) (*LivenessResult, error) {
	return &LivenessResult{Status: "ok"}, nil
}
