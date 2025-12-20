package devices

import (
	"context"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func retryInterceptor(maxRetries uint, cfg config.BackoffConfig) grpc.UnaryClientInterceptor {
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

		retryBackoff := backoff.WithMaxRetries(expBackoff, uint64(maxRetries))
		retryBackoff = backoff.WithContext(retryBackoff, ctx)

		var lastErr error
		operation := func() error {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				return nil
			}

			if isRetryable(err) {
				lastErr = err

				return err
			}

			return backoff.Permanent(err)
		}

		if err := backoff.Retry(operation, retryBackoff); err != nil {
			if lastErr != nil {
				return lastErr
			}

			return err
		}

		return nil
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
