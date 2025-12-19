package grpc

import (
	"context"

	"github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
)

type HealthHandler struct {
	devicev1.UnimplementedHealthServiceServer
	dbHealthChecker ports.DatabaseHealthChecker
}

func NewHealthHandler(dbHealthChecker ports.DatabaseHealthChecker) *HealthHandler {
	return &HealthHandler{dbHealthChecker: dbHealthChecker}
}

func (h *HealthHandler) Check(ctx context.Context, req *devicev1.HealthCheckRequest) (*devicev1.HealthCheckResponse, error) {
	if err := h.dbHealthChecker.Ping(ctx); err != nil {
		return &devicev1.HealthCheckResponse{
			Status: devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING,
		}, nil
	}

	return &devicev1.HealthCheckResponse{
		Status: devicev1.HealthCheckResponse_SERVING_STATUS_SERVING,
	}, nil
}

func (h *HealthHandler) Watch(req *devicev1.HealthCheckRequest, stream devicev1.HealthService_WatchServer) error {
	ctx := stream.Context()

	status := devicev1.HealthCheckResponse_SERVING_STATUS_SERVING
	if err := h.dbHealthChecker.Ping(ctx); err != nil {
		status = devicev1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING
	}

	return stream.Send(&devicev1.HealthCheckResponse{
		Status: status,
	})
}
