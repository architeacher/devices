package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-devices/internal/config"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	MetadataKeyRequestID     = "request-id"
	MetadataKeyCorrelationID = "correlation-id"
	MetadataKeyIdempotency   = "idempotency-key"

	ContextKeyRequestID     contextKey = "requestID"
	ContextKeyCorrelationID contextKey = "correlationID"
	ContextKeyIdempotency   contextKey = "idempotencyKey"

	healthServicePrefix = "HealthService"
)

func ContextExtractorInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var requestID string

		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if requestIDs := md.Get(MetadataKeyRequestID); len(requestIDs) > 0 {
				requestID = requestIDs[0]
			}

			if correlationIDs := md.Get(MetadataKeyCorrelationID); len(correlationIDs) > 0 {
				ctx = context.WithValue(ctx, ContextKeyCorrelationID, correlationIDs[0])
			}

			if idempotencyKeys := md.Get(MetadataKeyIdempotency); len(idempotencyKeys) > 0 {
				ctx = context.WithValue(ctx, ContextKeyIdempotency, idempotencyKeys[0])
			}
		}

		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)

		return handler(ctx, req)
	}
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return id
	}

	return ""
}

func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyCorrelationID).(string); ok {
		return id
	}

	return ""
}

func AccessLogInterceptor(log logger.Logger, cfg config.AccessLog) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !cfg.Enabled {
			return handler(ctx, req)
		}

		if !cfg.LogHealthChecks && isHealthCheck(info.FullMethod) {
			return handler(ctx, req)
		}

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		logEvent := log.Info().
			Str("method", info.FullMethod).
			Str("request_id", GetRequestID(ctx)).
			Dur("duration", duration)

		if correlationID := GetCorrelationID(ctx); correlationID != "" {
			logEvent = logEvent.Str("correlation_id", correlationID)
		}

		if cfg.IncludeMetadata {
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				logEvent = logEvent.Any("metadata", sanitizeMetadata(md))
			}
		}

		if err != nil {
			st, _ := status.FromError(err)
			logEvent.Str("grpc_code", st.Code().String()).
				Str("error", st.Message()).
				Msg("gRPC request failed")
		} else {
			logEvent.Msg("gRPC request completed")
		}

		return resp, err
	}
}

func isHealthCheck(fullMethod string) bool {
	return strings.Contains(fullMethod, healthServicePrefix)
}

func sanitizeMetadata(md metadata.MD) map[string]string {
	sanitized := make(map[string]string, len(md))
	sensitiveKeys := map[string]struct{}{
		"authorization": {},
		"paseto-token":  {},
		"api-key":       {},
		"cookie":        {},
	}

	for key, values := range md {
		if _, sensitive := sensitiveKeys[strings.ToLower(key)]; sensitive {
			sanitized[key] = "[REDACTED]"

			continue
		}

		if len(values) > 0 {
			sanitized[key] = values[0]
		}
	}

	return sanitized
}
