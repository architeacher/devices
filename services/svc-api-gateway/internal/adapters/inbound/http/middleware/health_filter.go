package middleware

import (
	"context"
	"net/http"
	"strings"
)

const (
	skipAccessLogKey contextKey = "skip_access_log"
)

var defaultHealthEndpoints = []string{
	"/health",
	"/healthz",
	"/liveness",
	"/readiness",
	"/ready",
	"/live",
	"/v1/health",
	"/v1/liveness",
	"/v1/readiness",
}

type HealthCheckFilter struct {
	healthEndpoints []string
	logHealthChecks bool
}

func NewHealthCheckFilter(logHealthChecks bool) *HealthCheckFilter {
	return &HealthCheckFilter{
		healthEndpoints: defaultHealthEndpoints,
		logHealthChecks: logHealthChecks,
	}
}

func (h *HealthCheckFilter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.logHealthChecks {
			next.ServeHTTP(w, r)

			return
		}

		if h.isHealthEndpoint(r.URL.Path) {
			ctx := context.WithValue(r.Context(), skipAccessLogKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))

			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *HealthCheckFilter) isHealthEndpoint(path string) bool {
	normalizedPath := strings.TrimSuffix(path, "/")

	for _, endpoint := range h.healthEndpoints {
		if normalizedPath == endpoint || normalizedPath == strings.TrimSuffix(endpoint, "/") {
			return true
		}
	}

	return false
}

func ShouldSkipAccessLog(ctx context.Context) bool {
	skip, ok := ctx.Value(skipAccessLogKey).(bool)

	return ok && skip
}
