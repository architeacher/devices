package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	MetadataKeyRequestID     = "request-id"
	MetadataKeyCorrelationID = "correlation-id"

	ContextKeyRequestID     contextKey = "requestID"
	ContextKeyCorrelationID contextKey = "correlationID"
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
