package devices

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type (
	// Client implements the DevicesService and HealthChecker interfaces using gRPC.
	Client struct {
		deviceClient devicev1.DeviceServiceClient
		healthClient devicev1.HealthServiceClient
		conn         *grpc.ClientConn
		cb           *gobreaker.CircuitBreaker[any]
		config       config.DevicesConfig
	}

	ClientOption func(*Client) error
)

var (
	_ ports.DevicesService = (*Client)(nil)
	_ ports.HealthChecker  = (*Client)(nil)
)

// NewClient creates a new gRPC client for the devices service.
func NewClient(cfg config.DevicesConfig, backoffCfg config.BackoffConfig, opts ...ClientOption) (*Client, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(4*1024*1024),
			grpc.MaxCallSendMsgSize(4*1024*1024),
		),
	}

	if cfg.TLS.Enabled {
		creds, err := loadTLSCredentials(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS credentials: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	dialOpts = append(dialOpts,
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			correlationIDInterceptor(),
			requestIDInterceptor(),
			timeoutInterceptor(cfg.Timeout),
			retryInterceptor(cfg.MaxRetries, backoffCfg),
		),
	)

	conn, err := grpc.NewClient(cfg.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC connection: %w", err)
	}

	client := &Client{
		deviceClient: devicev1.NewDeviceServiceClient(conn),
		healthClient: devicev1.NewHealthServiceClient(conn),
		conn:         conn,
		config:       cfg,
	}

	if cfg.CircuitBreaker.Enabled {
		client.cb = gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
			Name:        "devices-service",
			MaxRequests: uint32(cfg.CircuitBreaker.MaxRequests),
			Interval:    cfg.CircuitBreaker.Interval,
			Timeout:     cfg.CircuitBreaker.Timeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= uint32(cfg.CircuitBreaker.FailureThreshold)
			},
		})
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			conn.Close()

			return nil, fmt.Errorf("applying client option: %w", err)
		}
	}

	return client, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil

	return err
}

// CreateDevice creates a new device.
func (c *Client) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	req := &devicev1.CreateDeviceRequest{
		Name:  name,
		Brand: brand,
		State: toProtoState(state),
	}

	result, err := c.executeWithCircuitBreaker(ctx, func() (any, error) {
		return c.deviceClient.CreateDevice(ctx, req)
	})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	resp, ok := result.(*devicev1.CreateDeviceResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// GetDevice retrieves a device by ID.
func (c *Client) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	req := &devicev1.GetDeviceRequest{
		Id: id.String(),
	}

	result, err := c.executeWithCircuitBreaker(ctx, func() (any, error) {
		return c.deviceClient.GetDevice(ctx, req)
	})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	resp, ok := result.(*devicev1.GetDeviceResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// ListDevices retrieves a paginated list of devices with optional filters.
func (c *Client) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	req := toProtoListRequest(filter)

	result, err := c.executeWithCircuitBreaker(ctx, func() (any, error) {
		return c.deviceClient.ListDevices(ctx, req)
	})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	resp, ok := result.(*devicev1.ListDevicesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	return &model.DeviceList{
		Devices:    toDomainDevices(resp.GetDevices()),
		Pagination: toDomainPagination(resp.GetPagination()),
		Filters:    filter,
	}, nil
}

// UpdateDevice fully updates a device.
func (c *Client) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	req := &devicev1.UpdateDeviceRequest{
		Id:    id.String(),
		Name:  name,
		Brand: brand,
		State: toProtoState(state),
	}

	result, err := c.executeWithCircuitBreaker(ctx, func() (any, error) {
		return c.deviceClient.UpdateDevice(ctx, req)
	})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	resp, ok := result.(*devicev1.UpdateDeviceResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// PatchDevice partially updates a device.
func (c *Client) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	req := toProtoPatchRequest(id, updates)

	result, err := c.executeWithCircuitBreaker(ctx, func() (any, error) {
		return c.deviceClient.PatchDevice(ctx, req)
	})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	resp, ok := result.(*devicev1.PatchDeviceResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	return toDomainDevice(resp.GetDevice()), nil
}

// DeleteDevice deletes a device by ID.
func (c *Client) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	req := &devicev1.DeleteDeviceRequest{
		Id: id.String(),
	}

	_, err := c.executeWithCircuitBreaker(ctx, func() (any, error) {
		return c.deviceClient.DeleteDevice(ctx, req)
	})
	if err != nil {
		return mapGRPCError(err)
	}

	return nil
}

// Liveness returns the liveness status.
func (c *Client) Liveness(ctx context.Context) (*model.LivenessReport, error) {
	resp, err := c.healthClient.Check(ctx, &devicev1.HealthCheckRequest{})
	if err != nil {
		return &model.LivenessReport{
			Status:    model.HealthStatusDown,
			Timestamp: time.Now().UTC(),
			Version:   config.ServiceVersion,
		}, nil
	}

	status := model.HealthStatusOK
	if resp.GetStatus() != devicev1.HealthCheckResponse_SERVING_STATUS_SERVING {
		status = model.HealthStatusDown
	}

	return &model.LivenessReport{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Version:   config.ServiceVersion,
	}, nil
}

// Readiness returns the readiness status including dependency checks.
func (c *Client) Readiness(ctx context.Context) (*model.ReadinessReport, error) {
	checks := make(map[string]model.DependencyCheck)
	now := time.Now().UTC()

	resp, err := c.healthClient.Check(ctx, &devicev1.HealthCheckRequest{})
	if err != nil {
		checks["svc-devices"] = model.DependencyCheck{
			Status:      model.DependencyStatusDown,
			Message:     err.Error(),
			LastChecked: now,
		}

		return &model.ReadinessReport{
			Status:    model.HealthStatusDown,
			Timestamp: now,
			Version:   config.ServiceVersion,
			Checks:    checks,
		}, nil
	}

	depStatus := model.DependencyStatusUp
	if resp.GetStatus() != devicev1.HealthCheckResponse_SERVING_STATUS_SERVING {
		depStatus = model.DependencyStatusDown
	}

	checks["svc-devices"] = model.DependencyCheck{
		Status:      depStatus,
		Message:     "ok",
		LastChecked: now,
	}

	overallStatus := model.HealthStatusOK
	if depStatus == model.DependencyStatusDown {
		overallStatus = model.HealthStatusDown
	}

	return &model.ReadinessReport{
		Status:    overallStatus,
		Timestamp: now,
		Version:   config.ServiceVersion,
		Checks:    checks,
	}, nil
}

// Health returns a comprehensive health report.
func (c *Client) Health(ctx context.Context) (*model.HealthReport, error) {
	checks := make(map[string]model.DependencyCheck)
	now := time.Now().UTC()

	resp, err := c.healthClient.Check(ctx, &devicev1.HealthCheckRequest{})
	if err != nil {
		checks["svc-devices"] = model.DependencyCheck{
			Status:      model.DependencyStatusDown,
			Message:     err.Error(),
			LastChecked: now,
		}

		return &model.HealthReport{
			Status:    model.HealthStatusDown,
			Timestamp: now,
			Version: model.VersionInfo{
				API:   config.APIVersion,
				Build: config.CommitSHA,
			},
			Checks: checks,
		}, nil
	}

	depStatus := model.DependencyStatusUp
	if resp.GetStatus() != devicev1.HealthCheckResponse_SERVING_STATUS_SERVING {
		depStatus = model.DependencyStatusDown
	}

	checks["svc-devices"] = model.DependencyCheck{
		Status:      depStatus,
		Message:     "ok",
		LastChecked: now,
	}

	overallStatus := model.HealthStatusOK
	if depStatus == model.DependencyStatusDown {
		overallStatus = model.HealthStatusDown
	}

	return &model.HealthReport{
		Status:    overallStatus,
		Timestamp: now,
		Version: model.VersionInfo{
			API:   config.APIVersion,
			Build: config.CommitSHA,
		},
		Checks: checks,
	}, nil
}

func (c *Client) executeWithCircuitBreaker(_ context.Context, fn func() (any, error)) (any, error) {
	if c.cb == nil {
		return fn()
	}

	result, err := c.cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, model.ErrCircuitOpen
		}

		return nil, err
	}

	return result, nil
}

// WithConnection allows injecting an existing gRPC connection for testing.
func WithConnection(conn *grpc.ClientConn) ClientOption {
	return func(c *Client) error {
		if c.conn != nil {
			c.conn.Close()
		}

		c.conn = conn
		c.deviceClient = devicev1.NewDeviceServiceClient(conn)
		c.healthClient = devicev1.NewHealthServiceClient(conn)

		return nil
	}
}

func loadTLSCredentials(cfg config.TLSConfig) (credentials.TransportCredentials, error) {
	if cfg.CAFile == "" {
		return credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}), nil
	}

	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA file: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate")
	}

	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	if cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.CertFile)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}
