// Package noop provides a no-operation metrics client implementation
// for use in testing or when metrics collection is disabled.
package noop

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
)

type (
	MetricsClient struct{}
)

func NewMetricsClient() MetricsClient {
	return MetricsClient{}
}

func (c MetricsClient) Inc(_ context.Context, _ string, _ any, _ ...attribute.KeyValue) {}

func (c MetricsClient) Handler() http.Handler {
	return http.NotFoundHandler()
}

func (c MetricsClient) Shutdown(_ context.Context) error {
	return nil
}
