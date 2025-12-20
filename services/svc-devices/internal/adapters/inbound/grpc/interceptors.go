package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	MetadataKeyRequestID             = "x-request-id"
	ContextKeyRequestID  contextKey = "requestID"
)

func RequestIDExtractorInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if requestIDs := md.Get(MetadataKeyRequestID); len(requestIDs) > 0 {
				ctx = context.WithValue(ctx, ContextKeyRequestID, requestIDs[0])
			}
		}

		return handler(ctx, req)
	}
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return id
	}

	return ""
}
