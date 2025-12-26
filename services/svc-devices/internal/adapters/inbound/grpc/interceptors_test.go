package grpc_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/config"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestContextExtractorInterceptor_ExtractsRequestID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		metadata          metadata.MD
		expectedRequestID string
	}{
		{
			name:              "extracts request ID from metadata",
			metadata:          metadata.Pairs(inboundgrpc.MetadataKeyRequestID, "test-request-123"),
			expectedRequestID: "test-request-123",
		},
		{
			name:              "extracts UUID format request ID",
			metadata:          metadata.Pairs(inboundgrpc.MetadataKeyRequestID, "550e8400-e29b-41d4-a716-446655440000"),
			expectedRequestID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:              "uses first value when multiple request IDs present",
			metadata:          metadata.Pairs(inboundgrpc.MetadataKeyRequestID, "first-id", inboundgrpc.MetadataKeyRequestID, "second-id"),
			expectedRequestID: "first-id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			interceptor := inboundgrpc.ContextExtractorInterceptor()

			ctx := metadata.NewIncomingContext(t.Context(), tc.metadata)

			var capturedCtx context.Context
			mockHandler := func(ctx context.Context, req any) (any, error) {
				capturedCtx = ctx

				return "response", nil
			}

			resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
			require.NoError(t, err)
			require.Equal(t, "response", resp)

			extractedRequestID := inboundgrpc.GetRequestID(capturedCtx)
			require.Equal(t, tc.expectedRequestID, extractedRequestID)
		})
	}
}

func TestContextExtractorInterceptor_ExtractsCorrelationID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                  string
		metadata              metadata.MD
		expectedCorrelationID string
	}{
		{
			name:                  "extracts correlation ID from metadata",
			metadata:              metadata.Pairs(inboundgrpc.MetadataKeyCorrelationID, "test-correlation-123"),
			expectedCorrelationID: "test-correlation-123",
		},
		{
			name:                  "extracts UUID format correlation ID",
			metadata:              metadata.Pairs(inboundgrpc.MetadataKeyCorrelationID, "550e8400-e29b-41d4-a716-446655440000"),
			expectedCorrelationID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:                  "handles missing correlation ID",
			metadata:              metadata.MD{},
			expectedCorrelationID: "",
		},
		{
			name:                  "handles nil metadata",
			metadata:              nil,
			expectedCorrelationID: "",
		},
		{
			name:                  "uses first value when multiple correlation IDs present",
			metadata:              metadata.Pairs(inboundgrpc.MetadataKeyCorrelationID, "first-id", inboundgrpc.MetadataKeyCorrelationID, "second-id"),
			expectedCorrelationID: "first-id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			interceptor := inboundgrpc.ContextExtractorInterceptor()

			ctx := t.Context()
			if tc.metadata != nil {
				ctx = metadata.NewIncomingContext(ctx, tc.metadata)
			}

			var capturedCtx context.Context
			mockHandler := func(ctx context.Context, req any) (any, error) {
				capturedCtx = ctx

				return "response", nil
			}

			resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
			require.NoError(t, err)
			require.Equal(t, "response", resp)

			extractedCorrelationID := inboundgrpc.GetCorrelationID(capturedCtx)
			require.Equal(t, tc.expectedCorrelationID, extractedCorrelationID)
		})
	}
}

func TestContextExtractorInterceptor_GeneratesRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	interceptor := inboundgrpc.ContextExtractorInterceptor()

	ctx := t.Context()

	var capturedCtx context.Context
	mockHandler := func(ctx context.Context, req any) (any, error) {
		capturedCtx = ctx

		return "response", nil
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)

	requestID := inboundgrpc.GetRequestID(capturedCtx)
	require.NotEmpty(t, requestID, "request ID should be generated when missing")
	require.Len(t, requestID, 36, "request ID should be a UUID")
}

func TestContextExtractorInterceptor_GeneratesUniqueRequestIDs(t *testing.T) {
	t.Parallel()

	interceptor := inboundgrpc.ContextExtractorInterceptor()

	var requestIDs []string

	for range 3 {
		ctx := t.Context()

		var capturedCtx context.Context
		mockHandler := func(ctx context.Context, req any) (any, error) {
			capturedCtx = ctx

			return "response", nil
		}

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
		require.NoError(t, err)

		requestIDs = append(requestIDs, inboundgrpc.GetRequestID(capturedCtx))
	}

	require.NotEqual(t, requestIDs[0], requestIDs[1], "request IDs should be unique")
	require.NotEqual(t, requestIDs[1], requestIDs[2], "request IDs should be unique")
	require.NotEqual(t, requestIDs[0], requestIDs[2], "request IDs should be unique")
}

func TestContextExtractorInterceptor_PropagatesHandlerError(t *testing.T) {
	t.Parallel()

	interceptor := inboundgrpc.ContextExtractorInterceptor()

	ctx := metadata.NewIncomingContext(
		t.Context(),
		metadata.Pairs(inboundgrpc.MetadataKeyCorrelationID, "test-id"),
	)

	expectedErr := grpc.ErrServerStopped
	mockHandler := func(ctx context.Context, req any) (any, error) {
		return nil, expectedErr
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, resp)
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	requestID := inboundgrpc.GetRequestID(ctx)
	require.Empty(t, requestID)
}

func TestGetCorrelationID_EmptyContext(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	correlationID := inboundgrpc.GetCorrelationID(ctx)
	require.Empty(t, correlationID)
}

func TestAccessLogInterceptor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		config         config.AccessLog
		fullMethod     string
		metadata       metadata.MD
		handlerErr     error
		expectLog      bool
		expectMetadata bool
		expectErrorLog bool
	}{
		{
			name: "logs request when enabled",
			config: config.AccessLog{
				Enabled:         true,
				LogHealthChecks: true,
				IncludeMetadata: false,
			},
			fullMethod: "/device.v1.DeviceService/GetDevice",
			expectLog:  true,
		},
		{
			name: "skips logging when disabled",
			config: config.AccessLog{
				Enabled: false,
			},
			fullMethod: "/device.v1.DeviceService/GetDevice",
			expectLog:  false,
		},
		{
			name: "skips health check when LogHealthChecks is false",
			config: config.AccessLog{
				Enabled:         true,
				LogHealthChecks: false,
			},
			fullMethod: "/device.v1.HealthService/Check",
			expectLog:  false,
		},
		{
			name: "logs health check when LogHealthChecks is true",
			config: config.AccessLog{
				Enabled:         true,
				LogHealthChecks: true,
			},
			fullMethod: "/device.v1.HealthService/Check",
			expectLog:  true,
		},
		{
			name: "includes metadata when IncludeMetadata is true",
			config: config.AccessLog{
				Enabled:         true,
				LogHealthChecks: true,
				IncludeMetadata: true,
			},
			fullMethod:     "/device.v1.DeviceService/GetDevice",
			metadata:       metadata.Pairs("x-custom-header", "custom-value"),
			expectLog:      true,
			expectMetadata: true,
		},
		{
			name: "logs error when handler returns error",
			config: config.AccessLog{
				Enabled:         true,
				LogHealthChecks: true,
			},
			fullMethod:     "/device.v1.DeviceService/GetDevice",
			handlerErr:     grpc.ErrServerStopped,
			expectLog:      true,
			expectErrorLog: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			log := logger.NewWithWriter("info", "json", &buf)

			interceptor := inboundgrpc.AccessLogInterceptor(log, tc.config)

			ctx := context.WithValue(t.Context(), inboundgrpc.ContextKeyRequestID, "test-request-id")
			if tc.metadata != nil {
				ctx = metadata.NewIncomingContext(ctx, tc.metadata)
			}

			mockHandler := func(ctx context.Context, req any) (any, error) {
				if tc.handlerErr != nil {
					return nil, tc.handlerErr
				}

				return "response", nil
			}

			info := &grpc.UnaryServerInfo{FullMethod: tc.fullMethod}
			resp, err := interceptor(ctx, nil, info, mockHandler)

			if tc.handlerErr != nil {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, "response", resp)
			}

			logOutput := buf.String()

			if tc.expectLog {
				require.Contains(t, logOutput, tc.fullMethod)
				require.Contains(t, logOutput, "test-request-id")
			} else {
				require.Empty(t, logOutput)
			}

			if tc.expectMetadata {
				require.Contains(t, logOutput, "metadata")
				require.Contains(t, logOutput, "x-custom-header")
			}

			if tc.expectErrorLog {
				require.Contains(t, logOutput, "grpc_code")
				require.Contains(t, logOutput, "failed")
			}
		})
	}
}

func TestAccessLogInterceptor_SanitizesMetadata(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logger.NewWithWriter("info", "json", &buf)

	cfg := config.AccessLog{
		Enabled:         true,
		LogHealthChecks: true,
		IncludeMetadata: true,
	}

	interceptor := inboundgrpc.AccessLogInterceptor(log, cfg)

	ctx := context.WithValue(t.Context(), inboundgrpc.ContextKeyRequestID, "test-request-id")
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
		"authorization", "Bearer secret-token",
		"api-key", "api-key-value",
		"paseto-token", "paseto-token-value",
		"cookie", "session=abc123",
		"x-safe-header", "safe-value",
	))

	mockHandler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/device.v1.DeviceService/GetDevice"}
	_, err := interceptor(ctx, nil, info, mockHandler)
	require.NoError(t, err)

	logOutput := buf.String()

	require.Contains(t, logOutput, "[REDACTED]")
	require.NotContains(t, logOutput, "secret-token")
	require.NotContains(t, logOutput, "api-key-value")
	require.NotContains(t, logOutput, "paseto-token-value")
	require.NotContains(t, logOutput, "abc123")
	require.Contains(t, logOutput, "safe-value")
}

func TestAccessLogInterceptor_IncludesCorrelationID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logger.NewWithWriter("info", "json", &buf)

	cfg := config.AccessLog{
		Enabled:         true,
		LogHealthChecks: true,
	}

	interceptor := inboundgrpc.AccessLogInterceptor(log, cfg)

	ctx := context.WithValue(t.Context(), inboundgrpc.ContextKeyRequestID, "test-request-id")
	ctx = context.WithValue(ctx, inboundgrpc.ContextKeyCorrelationID, "test-correlation-id")

	mockHandler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/device.v1.DeviceService/GetDevice"}
	_, err := interceptor(ctx, nil, info, mockHandler)
	require.NoError(t, err)

	logOutput := buf.String()
	require.Contains(t, logOutput, "test-correlation-id")
}
