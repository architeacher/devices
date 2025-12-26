package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/architeacher/devices/pkg/logger"
)

const (
	skipAccessLogKey contextKey = "skip_access_log"
)

func AccessLogger(log logger.Logger, includeQueryParams bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipAccessLog(r.Context()) {
				next.ServeHTTP(w, r)

				return
			}

			start := time.Now()
			wrapped := NewFlushableResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			reqLogger := log.WithContext(r.Context()).
				With().
				Str("component", "http").
				Logger()

			event := reqLogger.Info()
			if wrapped.StatusCode() >= http.StatusInternalServerError {
				event = reqLogger.Error()
			} else if wrapped.StatusCode() >= http.StatusBadRequest {
				event = reqLogger.Warn()
			}

			event.
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Str("proto", r.Proto).
				Str("host", r.Host).
				Int("status", wrapped.StatusCode()).
				Uint64("bytes", wrapped.BytesWritten()).
				Int64("duration_ms", duration.Milliseconds())

			if includeQueryParams && r.URL.RawQuery != "" {
				event.Str("query", r.URL.RawQuery)
			}

			if referer := r.Referer(); referer != "" {
				event.Str("referer", referer)
			}

			event.Send()
		})
	}
}

func shouldSkipAccessLog(ctx context.Context) bool {
	skip, ok := ctx.Value(skipAccessLogKey).(bool)

	return ok && skip
}
