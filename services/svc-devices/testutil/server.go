// Package testutil provides test utilities for integration testing.
package testutil

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/services"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
)

const (
	postgresImage        = "postgres:18-alpine"
	postgresDatabase     = "devices_test"
	postgresUsername     = "test"
	postgresPassword     = "test"
	postgresNetworkAlias = "postgres"
	migrateImage         = "migrate/migrate:v4.19.1"
)

// TestServer provides a gRPC server with PostgreSQL for integration testing.
type TestServer struct {
	GRPCServer    *grpc.Server
	GRPCListener  net.Listener
	DBPool        *pgxpool.Pool
	Container     *postgres.PostgresContainer
	containerCtx  context.Context
	containerStop context.CancelFunc
}

// Option configures a TestServer.
type Option func(*config)

type config struct {
	migrationsPath string
}

// WithMigrationsPath sets a custom migrations path.
func WithMigrationsPath(path string) Option {
	return func(c *config) {
		c.migrationsPath = path
	}
}

// New creates a new TestServer with PostgreSQL container and gRPC server.
func New(ctx context.Context, opts ...Option) (*TestServer, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	containerCtx, containerStop := context.WithTimeout(ctx, 5*time.Minute)

	testNetwork, err := network.New(containerCtx)
	if err != nil {
		containerStop()

		return nil, fmt.Errorf("creating test network: %w", err)
	}

	container, err := postgres.Run(containerCtx,
		postgresImage,
		postgres.WithDatabase(postgresDatabase),
		postgres.WithUsername(postgresUsername),
		postgres.WithPassword(postgresPassword),
		network.WithNetwork([]string{postgresNetworkAlias}, testNetwork),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		containerStop()

		return nil, fmt.Errorf("starting postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(containerCtx, "sslmode=disable")
	if err != nil {
		container.Terminate(containerCtx)
		containerStop()

		return nil, fmt.Errorf("getting connection string: %w", err)
	}

	pool, err := pgxpool.New(containerCtx, connStr)
	if err != nil {
		container.Terminate(containerCtx)
		containerStop()

		return nil, fmt.Errorf("creating database pool: %w", err)
	}

	if err := runMigrations(containerCtx, testNetwork.Name, cfg.migrationsPath); err != nil {
		pool.Close()
		container.Terminate(containerCtx)
		containerStop()

		return nil, fmt.Errorf("running migrations: %w", err)
	}

	log := logger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()
	tracerProvider := otelNoop.NewTracerProvider()

	deviceRepo := repos.NewDevicesRepository(pool, repos.NewPgxScanner(), repos.NewCriteriaTranslator(&log), log)
	deviceSvc := services.NewDevicesService(deviceRepo)

	app := usecases.NewApplication(
		deviceSvc,
		deviceRepo,
		log,
		tracerProvider,
		metricsClient,
	)

	devicesHandler := inboundgrpc.NewDevicesHandler(app)
	healthHandler := inboundgrpc.NewHealthHandler(deviceRepo)

	grpcServer := grpc.NewServer()
	devicev1.RegisterDeviceServiceServer(grpcServer, devicesHandler)
	devicev1.RegisterHealthServiceServer(grpcServer, healthHandler)

	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pool.Close()
		container.Terminate(containerCtx)
		containerStop()

		return nil, fmt.Errorf("creating gRPC listener: %w", err)
	}

	go grpcServer.Serve(grpcListener)

	return &TestServer{
		GRPCServer:    grpcServer,
		GRPCListener:  grpcListener,
		DBPool:        pool,
		Container:     container,
		containerCtx:  containerCtx,
		containerStop: containerStop,
	}, nil
}

// Address returns the gRPC server address.
func (s *TestServer) Address() string {
	return s.GRPCListener.Addr().String()
}

// TruncateDevices removes all devices from the database.
func (s *TestServer) TruncateDevices(ctx context.Context) error {
	_, err := s.DBPool.Exec(ctx, "TRUNCATE TABLE devices")

	return err
}

// Close shuts down the server and cleans up resources.
func (s *TestServer) Close() {
	if s.GRPCServer != nil {
		s.GRPCServer.GracefulStop()
	}

	if s.DBPool != nil {
		s.DBPool.Close()
	}

	if s.Container != nil {
		s.Container.Terminate(s.containerCtx)
	}

	if s.containerStop != nil {
		s.containerStop()
	}
}

func runMigrations(ctx context.Context, networkName, customPath string) error {
	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=disable",
		postgresUsername,
		postgresPassword,
		postgresNetworkAlias,
		postgresDatabase,
	)

	migrationsPath := customPath
	var err error
	if migrationsPath == "" {
		migrationsPath, err = getMigrationsPath()
		if err != nil {
			return fmt.Errorf("getting migrations path: %w", err)
		}
	}

	migrateContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    migrateImage,
			Networks: []string{networkName},
			Cmd: []string{
				"-path", "/migrations",
				"-database", dbURL,
				"up",
			},
			Mounts: testcontainers.Mounts(
				testcontainers.BindMount(migrationsPath, "/migrations"),
			),
			WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return fmt.Errorf("running migrate container: %w", err)
	}
	defer migrateContainer.Terminate(ctx)

	state, err := migrateContainer.State(ctx)
	if err != nil {
		return fmt.Errorf("getting migrate container state: %w", err)
	}

	if state.ExitCode != 0 {
		logs, _ := migrateContainer.Logs(ctx)
		var logContent string
		if logs != nil {
			defer logs.Close()
			buf := make([]byte, 4096)
			n, _ := logs.Read(buf)
			logContent = string(buf[:n])
		}

		return fmt.Errorf("migrations failed with exit code %d, path: %s, logs: %s", state.ExitCode, migrationsPath, logContent)
	}

	return nil
}

func getMigrationsPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	serviceRoot := filepath.Dir(filepath.Dir(currentFile))

	return filepath.Join(serviceRoot, "migrations"), nil
}
