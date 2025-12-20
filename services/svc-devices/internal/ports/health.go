package ports

//counterfeiter:generate -o ../mocks/health_checker.go . HealthChecker
//counterfeiter:generate -o ../mocks/database_health_checker.go . DatabaseHealthChecker

import "context"

// HealthChecker defines the interface for health check operations.
type HealthChecker interface {
	// IsHealthy returns true if the service is healthy.
	IsHealthy(ctx context.Context) bool

	// CheckDependencies checks all service dependencies and returns their status.
	CheckDependencies(ctx context.Context) map[string]DependencyStatus
}

// DependencyStatus represents the health status of a dependency.
type DependencyStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// DatabaseHealthChecker defines the interface for database health checks.
type DatabaseHealthChecker interface {
	// Ping checks if the database connection is alive.
	Ping(ctx context.Context) error
}
