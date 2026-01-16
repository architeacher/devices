//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Package ports defines interface contracts for external dependencies.
package ports

// Generate mocks for proto-generated gRPC client interfaces.
// These interfaces are defined in the proto package, but we generate
// mocks here to keep all mocks in the service's internal/mocks directory.

//counterfeiter:generate -o ../mocks/grpc_device_client.go github.com/architeacher/devices/pkg/proto/device/v1.DeviceServiceClient
//counterfeiter:generate -o ../mocks/grpc_health_client.go github.com/architeacher/devices/pkg/proto/device/v1.HealthServiceClient
