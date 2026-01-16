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

// ApplyQueryDecoratorsWithCache applies all decorators including caching.
// Decorator order (outer to inner): logging → metrics → tracing → caching → base
// This ensures all requests (including cache hits) are logged, metriced, and traced.
func ApplyQueryDecoratorsWithCache[Q Query, R Result](
	handler QueryHandler[Q, R],
	cache Cache[Q, R],
	cacheConfig CacheConfig,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) QueryHandler[Q, R] {
	cachingHandler := NewQueryCachingDecorator(handler, cache, cacheConfig)

	return queryLoggingDecorator[Q, R]{
		base: queryMetricsDecorator[Q, R]{
			base: queryTracingDecorator[Q, R]{
				base:           cachingHandler,
				tracerProvider: tracerProvider,
			},
			client: metricsClient,
		},
		logger: log,
	}
}
