package decorator

import (
	"context"
	"fmt"
	"strings"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	Command any

	CommandHandler[C Command, R any] interface {
		Handle(context.Context, C) (R, error)
	}
)

func ApplyCommandDecorators[C Command, R any](
	handler CommandHandler[C, R],
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) CommandHandler[C, R] {
	return commandLoggingDecorator[C, R]{
		base: commandMetricsDecorator[C, R]{
			base: commandTracingDecorator[C, R]{
				base:           handler,
				tracerProvider: tracerProvider,
			},
			client: metricsClient,
		},
		logger: log,
	}
}

func generateActionName(handler any) string {
	return strings.Split(fmt.Sprintf("%T", handler), ".")[1]
}
