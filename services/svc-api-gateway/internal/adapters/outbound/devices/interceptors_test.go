package devices

import (
	"context"
	"strings"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDInterceptor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		requestID     string
		expectedInMD  bool
		expectedValue string
	}{
		{
			name:          "forwards valid request ID",
			requestID:     "test-request-id-123",
			expectedInMD:  true,
			expectedValue: "test-request-id-123",
		},
		{
			name:          "forwards UUID format request ID",
			requestID:     "550e8400-e29b-41d4-a716-446655440000",
			expectedInMD:  true,
			expectedValue: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:          "truncates long request ID",
			requestID:     strings.Repeat("a", 200),
			expectedInMD:  true,
			expectedValue: strings.Repeat("a", maxIDLength),
		},
		{
			name:          "truncates exactly at max length",
			requestID:     strings.Repeat("b", maxIDLength),
			expectedInMD:  true,
			expectedValue: strings.Repeat("b", maxIDLength),
		},
		{
			name:         "handles empty request ID",
			requestID:    "",
			expectedInMD: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			interceptor := requestIDInterceptor()

			ctx := t.Context()
			if tc.requestID != "" {
				ctx = context.WithValue(ctx, middleware.RequestIDKey, tc.requestID)
			}

			var capturedCtx context.Context
			mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				capturedCtx = ctx

				return nil
			}

			err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
			require.NoError(t, err)

			md, ok := metadata.FromOutgoingContext(capturedCtx)
			if tc.expectedInMD {
				require.True(t, ok, "expected metadata to be present")
				values := md.Get(MetadataKeyRequestID)
				require.Len(t, values, 1, "expected exactly one request ID value")
				require.Equal(t, tc.expectedValue, values[0])
			} else {
				if ok {
					values := md.Get(MetadataKeyRequestID)
					require.Empty(t, values, "expected no request ID in metadata")
				}
			}
		})
	}
}

func TestRequestIDInterceptor_PreservesExistingMetadata(t *testing.T) {
	t.Parallel()

	interceptor := requestIDInterceptor()

	ctx := t.Context()
	ctx = context.WithValue(ctx, middleware.RequestIDKey, "new-request-id")
	ctx = metadata.AppendToOutgoingContext(ctx, "existing-key", "existing-value")

	var capturedCtx context.Context
	mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtx = ctx

		return nil
	}

	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	require.NoError(t, err)

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok)
	require.Equal(t, []string{"existing-value"}, md.Get("existing-key"))
	require.Equal(t, []string{"new-request-id"}, md.Get(MetadataKeyRequestID))
}

func TestRequestIDInterceptor_PropagatesInvokerError(t *testing.T) {
	t.Parallel()

	interceptor := requestIDInterceptor()

	ctx := t.Context()
	ctx = context.WithValue(ctx, middleware.RequestIDKey, "test-id")

	expectedErr := grpc.ErrServerStopped
	mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return expectedErr
	}

	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	require.ErrorIs(t, err, expectedErr)
}

func TestCorrelationIDInterceptor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		correlationID string
		expectedInMD  bool
		expectedValue string
	}{
		{
			name:          "forwards valid correlation ID",
			correlationID: "test-correlation-id-123",
			expectedInMD:  true,
			expectedValue: "test-correlation-id-123",
		},
		{
			name:          "forwards UUID format correlation ID",
			correlationID: "550e8400-e29b-41d4-a716-446655440000",
			expectedInMD:  true,
			expectedValue: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:          "truncates long correlation ID",
			correlationID: strings.Repeat("a", 200),
			expectedInMD:  true,
			expectedValue: strings.Repeat("a", maxIDLength),
		},
		{
			name:         "handles empty correlation ID",
			correlationID: "",
			expectedInMD: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			interceptor := correlationIDInterceptor()

			ctx := t.Context()
			if tc.correlationID != "" {
				ctx = context.WithValue(ctx, middleware.CorrelationIDKey, tc.correlationID)
			}

			var capturedCtx context.Context
			mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				capturedCtx = ctx

				return nil
			}

			err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
			require.NoError(t, err)

			md, ok := metadata.FromOutgoingContext(capturedCtx)
			if tc.expectedInMD {
				require.True(t, ok, "expected metadata to be present")
				values := md.Get(MetadataKeyCorrelationID)
				require.Len(t, values, 1, "expected exactly one correlation ID value")
				require.Equal(t, tc.expectedValue, values[0])
			} else {
				if ok {
					values := md.Get(MetadataKeyCorrelationID)
					require.Empty(t, values, "expected no correlation ID in metadata")
				}
			}
		})
	}
}

func TestCorrelationIDInterceptor_PropagatesInvokerError(t *testing.T) {
	t.Parallel()

	interceptor := correlationIDInterceptor()

	ctx := t.Context()
	ctx = context.WithValue(ctx, middleware.CorrelationIDKey, "test-id")

	expectedErr := grpc.ErrServerStopped
	mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return expectedErr
	}

	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	require.ErrorIs(t, err, expectedErr)
}
