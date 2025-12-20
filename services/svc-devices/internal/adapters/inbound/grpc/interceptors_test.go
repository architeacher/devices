package grpc_test

import (
	"context"
	"testing"

	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDExtractorInterceptor(t *testing.T) {
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
			name:              "handles missing request ID",
			metadata:          metadata.MD{},
			expectedRequestID: "",
		},
		{
			name:              "handles nil metadata",
			metadata:          nil,
			expectedRequestID: "",
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

			interceptor := inboundgrpc.RequestIDExtractorInterceptor()

			ctx := context.Background()
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

			extractedID := inboundgrpc.GetRequestID(capturedCtx)
			require.Equal(t, tc.expectedRequestID, extractedID)
		})
	}
}

func TestRequestIDExtractorInterceptor_PropagatesHandlerError(t *testing.T) {
	t.Parallel()

	interceptor := inboundgrpc.RequestIDExtractorInterceptor()

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(inboundgrpc.MetadataKeyRequestID, "test-id"),
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

	ctx := context.Background()
	requestID := inboundgrpc.GetRequestID(ctx)
	require.Empty(t, requestID)
}
