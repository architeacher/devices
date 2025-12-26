package decorator

import (
	"context"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	Query  any
	Result any

	QueryHandler[Q Query, R Result] interface {
		Execute(ctx context.Context, query Q) (R, error)
	}
)

func ApplyQueryDecorators[Q Query, R Result](
	handler QueryHandler[Q, R],
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) QueryHandler[Q, R] {
	return queryLoggingDecorator[Q, R]{
		base: queryMetricsDecorator[Q, R]{
			base: queryTracingDecorator[Q, R]{
				base:           handler,
				tracerProvider: tracerProvider,
			},
			client: metricsClient,
		},
		logger: log,
	}
}
