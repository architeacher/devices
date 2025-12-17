package metrics

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type (
	Client interface {
		Inc(ctx context.Context, key string, value any, attributes ...attribute.KeyValue)
		Handler() http.Handler
		Shutdown(ctx context.Context) error
	}

	// Descriptor defines metadata used when registering OTEL instruments.
	Descriptor struct {
		Description string
		Unit        string
	}
)

// RegisterInt64Counter creates an Int64 counter using the provided descriptor map.
func RegisterInt64Counter(m metric.Meter, descriptor Descriptor, name string) (metric.Int64Counter, error) {
	counter, err := m.Int64Counter(
		name,
		metric.WithDescription(descriptor.Description),
		metric.WithUnit(descriptor.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s counter: %w", name, err)
	}

	return counter, nil
}

// RegisterFloat64Histogram creates a Float64 histogram using the provided descriptor map.
func RegisterFloat64Histogram(m metric.Meter, descriptor Descriptor, name string) (metric.Float64Histogram, error) {
	histogram, err := m.Float64Histogram(
		name,
		metric.WithDescription(descriptor.Description),
		metric.WithUnit(descriptor.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s histogram: %w", name, err)
	}

	return histogram, nil
}

// RegisterInt64Histogram creates an Int64 histogram using the provided descriptor map.
func RegisterInt64Histogram(m metric.Meter, descriptor Descriptor, name string) (metric.Int64Histogram, error) {
	histogram, err := m.Int64Histogram(
		name,
		metric.WithDescription(descriptor.Description),
		metric.WithUnit(descriptor.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s histogram: %w", name, err)
	}

	return histogram, nil
}

// RegisterInt64Gauge creates an Int64 gauge using the provided descriptor map.
func RegisterInt64Gauge(m metric.Meter, descriptor Descriptor, name string) (metric.Int64Gauge, error) {
	gauge, err := m.Int64Gauge(
		name,
		metric.WithDescription(descriptor.Description),
		metric.WithUnit(descriptor.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gauge: %w", name, err)
	}

	return gauge, nil
}
