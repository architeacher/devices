package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func Tracer() func(next http.Handler) http.Handler {
	return otelhttp.NewMiddleware(
		"http-server",
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.URL.Path
		}),
	)
}
