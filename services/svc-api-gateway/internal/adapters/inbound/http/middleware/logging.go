package middleware

import (
	"net/http"
	"time"

	"github.com/architeacher/devices/pkg/logger"
)

type AccessLogger struct {
	logger logger.Logger
}

func NewAccessLogger(log logger.Logger) *AccessLogger {
	return &AccessLogger{
		logger: log,
	}
}

func (a *AccessLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ShouldSkipAccessLog(r.Context()) {
			next.ServeHTTP(w, r)

			return
		}

		start := time.Now()
		wrapped := NewFlushableResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		requestID := GetRequestID(r.Context())

		event := a.logger.Info()
		if wrapped.StatusCode() >= http.StatusInternalServerError {
			event = a.logger.Error()
		} else if wrapped.StatusCode() >= http.StatusBadRequest {
			event = a.logger.Warn()
		}

		event.
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Str("proto", r.Proto).
			Str("host", r.Host).
			Int("status", wrapped.StatusCode()).
			Int64("bytes", wrapped.BytesWritten()).
			Int64("duration_ms", duration.Milliseconds()).
			Msg("request completed")
	})
}

// Logging returns a middleware that logs HTTP requests.
// Deprecated: Use NewAccessLogger instead for better configurability.
func Logging(log logger.Logger) func(http.Handler) http.Handler {
	accessLogger := NewAccessLogger(log)

	return accessLogger.Middleware
}
