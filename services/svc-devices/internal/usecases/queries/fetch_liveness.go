package queries

import (
	"context"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
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
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) FetchLivenessQueryHandler {
	return decorator.ApplyQueryDecorators[FetchLivenessQuery, *LivenessResult](
		fetchLivenessQueryHandler{},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h fetchLivenessQueryHandler) Execute(_ context.Context, _ FetchLivenessQuery) (*LivenessResult, error) {
	return &LivenessResult{Status: "ok"}, nil
}
