package ports

import (
	"context"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
)

type HealthChecker interface {
	Liveness(ctx context.Context) (*model.LivenessReport, error)
	Readiness(ctx context.Context) (*model.ReadinessReport, error)
	Health(ctx context.Context) (*model.HealthReport, error)
}
