package grpc

import (
	"context"

	"github.com/architeacher/devices/pkg/circuitbreaker"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client is a thin gRPC adapter that wraps a connection and makes protocol calls.
// Domain mapping and error handling are done by the service layer.
type Client struct {
	conn         *grpc.ClientConn
	deviceClient devicev1.DeviceServiceClient
	healthClient devicev1.HealthServiceClient
	cb           *circuitbreaker.CircuitBreaker[any]
	config       *config.ServiceConfig
}

// NewClient creates a new gRPC client wrapping the provided connection.
// The connection lifecycle is managed by the caller.
func NewClient(conn *grpc.ClientConn, cfg *config.ServiceConfig, opts ...Option) *Client {
	client := &Client{
		conn:   conn,
		config: cfg,
	}

	for _, opt := range opts {
		opt(client)
	}

	// Set defaults for anything not provided via options
	if client.deviceClient == nil {
		client.deviceClient = devicev1.NewDeviceServiceClient(conn)
	}

	if client.healthClient == nil {
		client.healthClient = devicev1.NewHealthServiceClient(conn)
	}

	if client.cb == nil {
		client.cb = circuitbreaker.New[any](circuitbreaker.Config{
			Name:             "svc-devices",
			Enabled:          cfg.DevicesGRPCClient.CircuitBreaker.Enabled,
			MaxRequests:      cfg.DevicesGRPCClient.CircuitBreaker.MaxRequests,
			Interval:         cfg.DevicesGRPCClient.CircuitBreaker.Interval,
			Timeout:          cfg.DevicesGRPCClient.CircuitBreaker.Timeout,
			FailureThreshold: cfg.DevicesGRPCClient.CircuitBreaker.FailureThreshold,
		})
	}

	return client
}

// Config returns the service configuration.
func (c *Client) Config() *config.ServiceConfig {
	return c.config
}

// --- Device Operations ---

// CreateDevice makes an gRPC call to create a device.
func (c *Client) CreateDevice(ctx context.Context, req *devicev1.CreateDeviceRequest) (*devicev1.CreateDeviceResponse, error) {
	result, err := circuitbreaker.Execute(c.cb, func() (any, error) {
		return c.deviceClient.CreateDevice(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return result.(*devicev1.CreateDeviceResponse), nil
}

// GetDevice makes an gRPC call to get a device.
func (c *Client) GetDevice(ctx context.Context, req *devicev1.GetDeviceRequest) (*devicev1.GetDeviceResponse, error) {
	result, err := circuitbreaker.Execute(c.cb, func() (any, error) {
		return c.deviceClient.GetDevice(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return result.(*devicev1.GetDeviceResponse), nil
}

// ListDevices makes a gRPC call to list devices.
func (c *Client) ListDevices(ctx context.Context, req *devicev1.ListDevicesRequest) (*devicev1.ListDevicesResponse, error) {
	result, err := circuitbreaker.Execute(c.cb, func() (any, error) {
		return c.deviceClient.ListDevices(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return result.(*devicev1.ListDevicesResponse), nil
}

// UpdateDevice makes a gRPC call to update a device.
func (c *Client) UpdateDevice(ctx context.Context, req *devicev1.UpdateDeviceRequest) (*devicev1.UpdateDeviceResponse, error) {
	result, err := circuitbreaker.Execute(c.cb, func() (any, error) {
		return c.deviceClient.UpdateDevice(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return result.(*devicev1.UpdateDeviceResponse), nil
}

// PatchDevice makes a gRPC call to patch a device.
func (c *Client) PatchDevice(ctx context.Context, req *devicev1.PatchDeviceRequest) (*devicev1.PatchDeviceResponse, error) {
	result, err := circuitbreaker.Execute(c.cb, func() (any, error) {
		return c.deviceClient.PatchDevice(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return result.(*devicev1.PatchDeviceResponse), nil
}

// DeleteDevice makes a gRPC call to delete a device.
func (c *Client) DeleteDevice(ctx context.Context, req *devicev1.DeleteDeviceRequest) (*emptypb.Empty, error) {
	result, err := circuitbreaker.Execute(c.cb, func() (any, error) {
		return c.deviceClient.DeleteDevice(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	return result.(*emptypb.Empty), nil
}

// --- Health Operations ---

// CheckHealth makes a gRPC health check call.
func (c *Client) CheckHealth(ctx context.Context, req *devicev1.HealthCheckRequest) (*devicev1.HealthCheckResponse, error) {
	return c.healthClient.Check(ctx, req)
}
