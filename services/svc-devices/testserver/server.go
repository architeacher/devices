// Package testserver provides a test gRPC server for integration testing.
package testserver

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	devicev1 "github.com/architeacher/devices/pkg/proto/device/v1"
	inboundgrpc "github.com/architeacher/devices/services/svc-devices/internal/adapters/inbound/grpc"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/services"
	"github.com/architeacher/devices/services/svc-devices/internal/usecases"
	"github.com/architeacher/devices/services/svc-devices/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
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

// New creates a new TestServer with PostgreSQL container and gRPC server.
func New(ctx context.Context) (*TestServer, error) {
	containerCtx, containerStop := context.WithTimeout(ctx, 5*time.Minute)

	container, err := postgres.Run(containerCtx,
		"postgres:18-alpine",
		postgres.WithDatabase("devices_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
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

	if err := runMigrations(containerCtx, pool); err != nil {
		pool.Close()
		container.Terminate(containerCtx)
		containerStop()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	log := logger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()
	tracerProvider := otelNoop.NewTracerProvider()

	deviceRepo := repos.NewDevicesRepository(pool)
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

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	var migrationFiles []string

	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".up.sql") {
			migrationFiles = append(migrationFiles, path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	sort.Slice(migrationFiles, func(i, j int) bool {
		return filepath.Base(migrationFiles[i]) < filepath.Base(migrationFiles[j])
	})

	for _, file := range migrationFiles {
		content, err := migrations.FS.ReadFile(file)
		if err != nil {
			return err
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("executing migration %s: %w", file, err)
		}
	}

	return nil
}
