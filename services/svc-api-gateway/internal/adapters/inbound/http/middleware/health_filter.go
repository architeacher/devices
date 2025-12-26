package middleware

import (
	"context"
	"net/http"
	"strings"
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

func HealthCheckFilter(logHealthChecks bool) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logHealthChecks {
				next.ServeHTTP(w, r)

				return
			}

			if isHealthEndpoint(r.URL.Path, defaultHealthEndpoints) {
				ctx := context.WithValue(r.Context(), skipAccessLogKey, true)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isHealthEndpoint(path string, healthEndpoints []string) bool {
	for _, endpoint := range healthEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}

	return false
}
