package grpc

import (
	"github.com/architeacher/devices/pkg/circuitbreaker"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
)

// Option configures the gRPC Client.
type Option func(*Client)

// WithDeviceClient allows injecting a device service client for testing.
func WithDeviceClient(client devicev1.DeviceServiceClient) Option {
	return func(c *Client) {
		c.deviceClient = client
	}
}

// WithHealthClient allows injecting a health service client for testing.
func WithHealthClient(client devicev1.HealthServiceClient) Option {
	return func(c *Client) {
		c.healthClient = client
	}
}

// WithCircuitBreaker allows injecting a custom circuit breaker for testing.
func WithCircuitBreaker(cb *circuitbreaker.CircuitBreaker[any]) Option {
	return func(c *Client) {
		c.cb = cb
	}
}
