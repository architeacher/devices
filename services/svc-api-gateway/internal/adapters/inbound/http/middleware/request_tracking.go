package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDHeader     = "Request-Id"
	CorrelationIDHeader = "Correlation-Id"

	RequestIDKey     contextKey = "requestID"
	CorrelationIDKey contextKey = "correlationID"
)

func RequestTracking() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := r.Header.Get(CorrelationIDHeader)
			if correlationID == "" {
				correlationID = uuid.New().String()
			}

			requestID := r.Header.Get(RequestIDHeader)
			if requestID == "" {
				requestID = uuid.New().String()
			}

			ctx := context.WithValue(r.Context(), CorrelationIDKey, correlationID)
			ctx = context.WithValue(ctx, RequestIDKey, requestID)

			w.Header().Set(CorrelationIDHeader, correlationID)
			w.Header().Set(RequestIDHeader, requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}

	return ""
}

func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}

	return ""
}
