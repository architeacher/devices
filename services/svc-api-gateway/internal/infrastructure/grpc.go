package infrastructure

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/architeacher/devices/pkg/idempotency"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/cenkalti/backoff/v5"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	MetadataKeyRequestID     = "request-id"
	MetadataKeyCorrelationID = "correlation-id"
	MetadataKeyIdempotency   = "idempotency-key"
	maxIDLength              = 128
)

// NewGRPCConnection creates a new gRPC client connection with the configured options.
// The connection lifecycle is managed by the caller.
func NewGRPCConnection(cfg *config.ServiceConfig) (*grpc.ClientConn, error) {
	grpcClientConfig := cfg.DevicesGRPCClient

	dialOpts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(int(grpcClientConfig.MaxMessageSize)),
			grpc.MaxCallSendMsgSize(int(grpcClientConfig.MaxMessageSize)),
		),
	}

	if grpcClientConfig.TLS.Enabled {
		creds, err := loadTLSCredentials(grpcClientConfig.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS credentials: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	dialOpts = append(dialOpts,
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			correlationIDInterceptor(),
			requestIDInterceptor(),
			idempotencyInterceptor(),
			timeoutInterceptor(grpcClientConfig.Timeout),
			retryInterceptor(grpcClientConfig.MaxRetries, cfg.Backoff),
		),
	)

	conn, err := grpc.NewClient(grpcClientConfig.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC connection: %w", err)
	}

	return conn, nil
}

func loadTLSCredentials(cfg config.TLSConfig) (credentials.TransportCredentials, error) {
	if cfg.CAFile == "" {
		return credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}), nil
	}

	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA file: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate")
	}

	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	if cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.CertFile)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

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
