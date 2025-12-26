package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/architeacher/devices/pkg/decorator"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-devices/internal/config"
	"github.com/architeacher/devices/services/svc-devices/internal/ports"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	FetchHealthReportQuery struct{}

	HealthResult struct {
		Status       string                            `json:"status"`
		Version      string                            `json:"version"`
		Uptime       string                            `json:"uptime"`
		Dependencies map[string]ports.DependencyStatus `json:"dependencies"`
	}

	FetchHealthReportQueryHandler = decorator.QueryHandler[FetchHealthReportQuery, *HealthResult]

	fetchHealthReportQueryHandler struct {
		dbHealthChecker ports.DatabaseHealthChecker
		startTime       time.Time
	}
)

func NewFetchHealthReportQueryHandler(
	dbHealthChecker ports.DatabaseHealthChecker,
	log logger.Logger,
	metricsClient metrics.Client,
	tracerProvider otelTrace.TracerProvider,
) FetchHealthReportQueryHandler {
	return decorator.ApplyQueryDecorators[FetchHealthReportQuery, *HealthResult](
		fetchHealthReportQueryHandler{
			dbHealthChecker: dbHealthChecker,
			startTime:       time.Now(),
		},
		log,
		metricsClient,
		tracerProvider,
	)
}

func (h fetchHealthReportQueryHandler) Execute(ctx context.Context, _ FetchHealthReportQuery) (*HealthResult, error) {
	dependencies := make(map[string]ports.DependencyStatus)

	start := time.Now()
	dbErr := h.dbHealthChecker.Ping(ctx)
	latency := time.Since(start)

	dbStatus := ports.DependencyStatus{
		Healthy: dbErr == nil,
		Latency: fmt.Sprintf("%dms", latency.Milliseconds()),
	}

	if dbErr != nil {
		dbStatus.Message = dbErr.Error()
	}

	dependencies["postgres"] = dbStatus

	overallStatus := "healthy"
	if !dbStatus.Healthy {
		overallStatus = "unhealthy"
	}

	return &HealthResult{
		Status:       overallStatus,
		Version:      config.ServiceVersion,
		Uptime:       time.Since(h.startTime).String(),
		Dependencies: dependencies,
	}, nil
}
