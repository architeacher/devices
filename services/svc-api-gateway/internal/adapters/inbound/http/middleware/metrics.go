package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/architeacher/devices/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
)

const (
	httpMethodKey     = "http.method"
	httpPathKey       = "http.path"
	httpStatusCodeKey = "http.status_code"

	httpRequestTotal    = "http_requests_total"
	httpRequestDuration = "http_request_duration_seconds"
	httpRequestSize     = "http_request_size_bytes"
	httpResponseSize    = "http_response_size_bytes"
)

type MetricsMiddleware struct {
	metricsClient metrics.Client
}

func NewMetricsMiddleware(metricsClient metrics.Client) *MetricsMiddleware {
	return &MetricsMiddleware{
		metricsClient: metricsClient,
	}
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		wrapped := NewFlushableResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(startTime)

		requestSize := clampToUint64(r.ContentLength)
		responseSize := uint64(wrapped.BytesWritten())

		m.recordHTTPRequest(
			r.Context(),
			r.Method,
			r.URL.Path,
			uint(wrapped.StatusCode()),
			duration,
			requestSize,
			responseSize,
		)
	})
}

func (m *MetricsMiddleware) recordHTTPRequest(
	ctx context.Context,
	method, path string,
	statusCode uint,
	duration time.Duration,
	requestSize, responseSize uint64,
) {
	attrs := []attribute.KeyValue{
		attribute.String(httpMethodKey, method),
		attribute.String(httpPathKey, path),
		attribute.String(httpStatusCodeKey, fmt.Sprintf("%d", statusCode)),
	}

	m.metricsClient.Inc(ctx, httpRequestTotal, int64(1), attrs...)
	m.metricsClient.Inc(ctx, httpRequestDuration, duration.Seconds(), attrs...)

	m.metricsClient.Inc(
		ctx,
		httpRequestSize,
		int64(requestSize),
		attribute.String(httpMethodKey, method),
		attribute.String(httpPathKey, path),
	)

	m.metricsClient.Inc(ctx, httpResponseSize, int64(responseSize), attrs...)
}

func clampToUint64(value int64) uint64 {
	if value > 0 {
		return uint64(value)
	}

	return 0
}
