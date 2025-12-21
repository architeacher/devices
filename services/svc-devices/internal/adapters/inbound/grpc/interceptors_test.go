package grpc_test

import (
	"context"
	"testing"

	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
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
