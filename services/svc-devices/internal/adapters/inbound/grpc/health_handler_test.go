package grpc_test

import (
	"testing"

	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/architeacher/devices/services/svc-devices/internal/mocks"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Check(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		setupChecker   func(*mocks.FakeDatabaseHealthChecker)
		expectedStatus devicev1.HealthCheckResponse_ServingStatus
	}{
		{
			name: "service is serving when db is healthy",
			setupChecker: func(fake *mocks.FakeDatabaseHealthChecker) {
				fake.PingReturns(nil)
			},
			expectedStatus: devicev1.HealthCheckResponse_SERVING_STATUS_SERVING,
		},
		{
			name: "service is not serving when db is unhealthy",
			setupChecker: func(fake *mocks.FakeDatabaseHealthChecker) {
				fake.PingReturns(model.ErrDatabaseConnection)
			},
			expectedStatus: devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbChecker := &mocks.FakeDatabaseHealthChecker{}
			tc.setupChecker(dbChecker)
			handler := inboundgrpc.NewHealthHandler(dbChecker)

			resp, err := handler.Check(t.Context(), &devicev1.HealthCheckRequest{})

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tc.expectedStatus, resp.Status)
		})
	}
}
