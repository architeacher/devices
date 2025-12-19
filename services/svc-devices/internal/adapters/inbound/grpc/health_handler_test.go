package grpc_test

import (
	"context"
	"testing"

	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Check(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		dbHealthy      bool
		expectedStatus devicev1.HealthCheckResponse_ServingStatus
	}{
		{
			name:           "service is serving when db is healthy",
			dbHealthy:      true,
			expectedStatus: devicev1.HealthCheckResponse_SERVING_STATUS_SERVING,
		},
		{
			name:           "service is not serving when db is unhealthy",
			dbHealthy:      false,
			expectedStatus: devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbChecker := &mockHealthChecker{healthy: tc.dbHealthy}
			handler := inboundgrpc.NewHealthHandler(dbChecker)

			resp, err := handler.Check(context.Background(), &devicev1.HealthCheckRequest{})

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tc.expectedStatus, resp.Status)
		})
	}
}

type mockHealthChecker struct {
	healthy bool
}

func (m *mockHealthChecker) Ping(_ context.Context) error {
	if !m.healthy {
		return model.ErrDatabaseConnection
	}

	return nil
}
