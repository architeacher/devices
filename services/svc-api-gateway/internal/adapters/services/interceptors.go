package services

import (
	"context"
	"time"

	"github.com/architeacher/devices/pkg/idempotency"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/cenkalti/backoff/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	MetadataKeyRequestID     = "request-id"
	MetadataKeyCorrelationID = "correlation-id"
	MetadataKeyIdempotency   = "idempotency-key"
	maxIDLength              = 128
)

func correlationIDInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		correlationID := middleware.GetCorrelationID(ctx)
		if correlationID != "" {
			if len(correlationID) > maxIDLength {
				correlationID = correlationID[:maxIDLength]
			}

			ctx = metadata.AppendToOutgoingContext(ctx, MetadataKeyCorrelationID, correlationID)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func requestIDInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		requestID := middleware.GetRequestID(ctx)
		if requestID != "" {
			if len(requestID) > maxIDLength {
				requestID = requestID[:maxIDLength]
			}

			ctx = metadata.AppendToOutgoingContext(ctx, MetadataKeyRequestID, requestID)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func idempotencyInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		key, ok := idempotency.FromContext(ctx)
		if ok && key != "" {
			if len(key) > maxIDLength {
				key = key[:maxIDLength]
			}

			ctx = metadata.AppendToOutgoingContext(ctx, MetadataKeyIdempotency, key)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func timeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func retryInterceptor(maxRetries uint, cfg config.Backoff) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if maxRetries == 0 {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		expBackoff := backoff.NewExponentialBackOff()
		expBackoff.InitialInterval = cfg.BaseDelay
		expBackoff.Multiplier = cfg.Multiplier
		expBackoff.RandomizationFactor = cfg.Jitter
		expBackoff.MaxInterval = cfg.MaxDelay

		operation := func() (struct{}, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				return struct{}{}, nil
			}

			if isRetryable(err) {
				return struct{}{}, err
			}

			return struct{}{}, backoff.Permanent(err)
		}

		_, err := backoff.Retry(
			ctx,
			operation,
			backoff.WithMaxTries(maxRetries+1),
			backoff.WithBackOff(expBackoff),
		)

		return err
	}
}

func isRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch st.Code() {
	case codes.Unavailable, codes.ResourceExhausted, codes.Aborted:
		return true
	default:
		return false
	}
}
