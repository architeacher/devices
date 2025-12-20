package decorator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	commandTracingDecorator[C Command, R any] struct {
		base           CommandHandler[C, R]
		tracerProvider otelTrace.TracerProvider
	}

	queryTracingDecorator[Q Query, R Result] struct {
		base           QueryHandler[Q, R]
		tracerProvider otelTrace.TracerProvider
	}
)

//nolint:dupl // False positive.
func (d commandTracingDecorator[C, R]) Handle(ctx context.Context, cmd C) (result R, err error) {
	start := time.Now()

	defer func() {
		if d.tracerProvider == nil {
			return
		}

		actionName := strings.ToLower(generateActionName(cmd))

		_, span := d.tracerProvider.Tracer(fmt.Sprintf("commands.%s", actionName)).Start(ctx, "Handle")
		span.SetAttributes(attribute.String("duration", time.Since(start).String()))

		defer span.End()

		if err == nil {
			span.AddEvent(fmt.Sprintf("commands.%s.success", actionName))
			span.SetStatus(codes.Ok, fmt.Sprintf("commands.%s", actionName))

			return
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("commands.%s.failure", actionName))
		span.AddEvent(fmt.Sprintf("commands.%s.failure", actionName))
	}()

	return d.base.Handle(ctx, cmd)
}

func (d queryTracingDecorator[Q, R]) Execute(ctx context.Context, query Q) (result R, err error) {
	start := time.Now()

	defer func() {
		if d.tracerProvider == nil {
			return
		}

		actionName := strings.ToLower(generateActionName(query))

		_, span := d.tracerProvider.Tracer(fmt.Sprintf("queries.%s", actionName)).Start(ctx, "Handle")
		span.SetAttributes(attribute.String("duration", time.Since(start).String()))

		defer span.End()

		if err == nil {
			span.AddEvent(fmt.Sprintf("queries.%s.success", actionName))
			span.SetStatus(codes.Ok, fmt.Sprintf("queries.%s", actionName))

			return
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("queries.%s.failure", actionName))
		span.AddEvent(fmt.Sprintf("queries.%s.failure", actionName))
	}()

	return d.base.Execute(ctx, query)
}
