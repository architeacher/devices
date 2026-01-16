//go:build integration

package itest

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage    = "postgres:18-alpine"
	postgresDatabase = "devices_test"
	postgresUsername = "test"
	postgresPassword = "test"
	migrateImage     = "migrate/migrate:v4.19.1"
)

type DevicesRepositoryIntegrationTestSuite struct {
	suite.Suite
	suiteCtx    context.Context
	suiteCancel context.CancelFunc
	container   *postgres.PostgresContainer
	pool        *pgxpool.Pool
	repo        *repos.DevicesRepository
}

func TestDevicesRepositoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(DevicesRepositoryIntegrationTestSuite))
}

func (s *DevicesRepositoryIntegrationTestSuite) SetupSuite() {
	s.suiteCtx, s.suiteCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	container, err := postgres.Run(s.suiteCtx,
		postgresImage,
		postgres.WithDatabase(postgresDatabase),
		postgres.WithUsername(postgresUsername),
		postgres.WithPassword(postgresPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	s.Require().NoError(err)
	s.container = container

	connStr, err := container.ConnectionString(s.suiteCtx, "sslmode=disable")
	s.Require().NoError(err)

	pool, err := pgxpool.New(s.suiteCtx, connStr)
	s.Require().NoError(err)
	s.pool = pool

	s.runMigrations()

	log := logger.NewTestLogger()
	s.repo = repos.NewDevicesRepository(s.pool, repos.NewPgxScanner(), repos.NewCriteriaTranslator(&log), log)
}

func (s *DevicesRepositoryIntegrationTestSuite) TearDownSuite() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.suiteCtx)
	}
	if s.suiteCancel != nil {
		s.suiteCancel()
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) SetupTest() {
	ctx := s.T().Context()
	_, err := s.pool.Exec(ctx, "TRUNCATE TABLE devices")
	s.Require().NoError(err)
}

func (s *DevicesRepositoryIntegrationTestSuite) runMigrations() {
	postgresPort, err := s.container.MappedPort(s.suiteCtx, "5432/tcp")
	s.Require().NoError(err)

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@host.docker.internal:%s/%s?sslmode=disable",
		postgresUsername,
		postgresPassword,
		postgresPort.Port(),
		postgresDatabase,
	)

	migrationsPath, err := getMigrationsPath()
	s.Require().NoError(err)

	migrateContainer, err := testcontainers.GenericContainer(s.suiteCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: migrateImage,
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
	s.Require().NoError(err)
	defer migrateContainer.Terminate(s.suiteCtx)

	state, err := migrateContainer.State(s.suiteCtx)
	s.Require().NoError(err)
	s.Require().Equal(0, state.ExitCode, "migrations failed")
}

func getMigrationsPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	serviceRoot := filepath.Dir(filepath.Dir(currentFile))

	return filepath.Join(serviceRoot, "migrations"), nil
}

func (s *DevicesRepositoryIntegrationTestSuite) seedDevice(ctx context.Context, device *model.Device) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (id, name, brand, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, device.ID.String(), device.Name, device.Brand, device.State.String(),
		device.CreatedAt, device.UpdatedAt)
	s.Require().NoError(err)
}

func (s *DevicesRepositoryIntegrationTestSuite) seedDevices(ctx context.Context, devices []*model.Device) {
	for _, device := range devices {
		s.seedDevice(ctx, device)
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestCreate_Success() {
	ctx := s.T().Context()

	device := model.NewDevice("Test Device", "Test Brand", model.StateAvailable)

	err := s.repo.Create(ctx, device)

	s.Require().NoError(err)

	retrieved, err := s.repo.FetchByID(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().Equal(device.Name, retrieved.Name)
	s.Require().Equal(device.Brand, retrieved.Brand)
	s.Require().Equal(device.State, retrieved.State)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestCreate_AllStates() {
	ctx := s.T().Context()

	states := []model.State{model.StateAvailable, model.StateInUse, model.StateInactive}

	for _, state := range states {
		device := model.NewDevice(fmt.Sprintf("Device-%s", state), "Brand", state)
		err := s.repo.Create(ctx, device)
		s.Require().NoError(err)

		retrieved, err := s.repo.FetchByID(ctx, device.ID)
		s.Require().NoError(err)
		s.Require().Equal(state, retrieved.State)
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestCreate_DuplicateID() {
	ctx := s.T().Context()

	device := model.NewDevice("Original", "Brand", model.StateAvailable)
	err := s.repo.Create(ctx, device)
	s.Require().NoError(err)

	duplicate := &model.Device{
		ID:        device.ID,
		Name:      "Duplicate",
		Brand:     "Other Brand",
		State:     model.StateInactive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err = s.repo.Create(ctx, duplicate)
	s.Require().Error(err)
	s.Require().ErrorIs(err, model.ErrDuplicateDevice)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestGetByID_Success() {
	ctx := s.T().Context()

	device := model.NewDevice("Test Device", "Test Brand", model.StateAvailable)
	s.seedDevice(ctx, device)

	retrieved, err := s.repo.FetchByID(ctx, device.ID)

	s.Require().NoError(err)
	s.Require().NotNil(retrieved)
	s.Require().Equal(device.ID, retrieved.ID)
	s.Require().Equal(device.Name, retrieved.Name)
	s.Require().Equal(device.Brand, retrieved.Brand)
	s.Require().Equal(device.State, retrieved.State)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestGetByID_NotFound() {
	ctx := s.T().Context()

	nonExistentID := model.NewDeviceID()

	retrieved, err := s.repo.FetchByID(ctx, nonExistentID)

	s.Require().Error(err)
	s.Require().ErrorIs(err, model.ErrDeviceNotFound)
	s.Require().Nil(retrieved)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_Empty() {
	ctx := s.T().Context()

	list, err := s.repo.List(ctx, model.DefaultDeviceFilter())

	s.Require().NoError(err)
	s.Require().NotNil(list)
	s.Require().Empty(list.Devices)
	s.Require().Equal(uint(0), list.Pagination.TotalItems)
	s.Require().Equal(uint(0), list.Pagination.TotalPages)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_AllDevices() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("Device 1", "Brand A", model.StateAvailable),
		model.NewDevice("Device 2", "Brand B", model.StateInUse),
		model.NewDevice("Device 3", "Brand A", model.StateInactive),
	}
	s.seedDevices(ctx, devices)

	list, err := s.repo.List(ctx, model.DefaultDeviceFilter())

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal(uint(3), list.Pagination.TotalItems)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_FilterByBrand() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone", "Apple", model.StateAvailable),
		model.NewDevice("MacBook", "Apple", model.StateInUse),
		model.NewDevice("Galaxy", "Samsung", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Brands: []string{"Apple"},
		Page:   1,
		Size:   20,
		Sort:   []string{"-createdAt"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 2)
	for _, device := range list.Devices {
		s.Require().Equal("Apple", device.Brand)
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_FilterByState() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("Device 1", "Brand", model.StateAvailable),
		model.NewDevice("Device 2", "Brand", model.StateInUse),
		model.NewDevice("Device 3", "Brand", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		States: []model.State{model.StateAvailable},
		Page:   1,
		Size:   20,
		Sort:   []string{"-createdAt"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 2)
	for _, device := range list.Devices {
		s.Require().Equal(model.StateAvailable, device.State)
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_Pagination() {
	ctx := s.T().Context()

	for index := 0; index < 25; index++ {
		device := model.NewDevice(fmt.Sprintf("Device %d", index+1), "Brand", model.StateAvailable)
		time.Sleep(time.Millisecond)
		s.seedDevice(ctx, device)
	}

	filter := model.DeviceFilter{
		Page: 1,
		Size: 10,
		Sort: []string{"-createdAt"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 10)
	s.Require().Equal(uint(25), list.Pagination.TotalItems)
	s.Require().Equal(uint(3), list.Pagination.TotalPages)
	s.Require().True(list.Pagination.HasNext)
	s.Require().False(list.Pagination.HasPrevious)

	filter.Page = 2
	list, err = s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 10)
	s.Require().True(list.Pagination.HasNext)
	s.Require().True(list.Pagination.HasPrevious)

	filter.Page = 3
	list, err = s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 5)
	s.Require().False(list.Pagination.HasNext)
	s.Require().True(list.Pagination.HasPrevious)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_SortByName() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("Charlie", "Brand", model.StateAvailable),
		model.NewDevice("Alpha", "Brand", model.StateAvailable),
		model.NewDevice("Bravo", "Brand", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: []string{"name"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal("Alpha", list.Devices[0].Name)
	s.Require().Equal("Bravo", list.Devices[1].Name)
	s.Require().Equal("Charlie", list.Devices[2].Name)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_SortByNameDescending() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("Alpha", "Brand", model.StateAvailable),
		model.NewDevice("Charlie", "Brand", model.StateAvailable),
		model.NewDevice("Bravo", "Brand", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: []string{"-name"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal("Charlie", list.Devices[0].Name)
	s.Require().Equal("Bravo", list.Devices[1].Name)
	s.Require().Equal("Alpha", list.Devices[2].Name)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_SortByBrandDescending() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("Device 1", "Apple", model.StateAvailable),
		model.NewDevice("Device 2", "Samsung", model.StateAvailable),
		model.NewDevice("Device 3", "Google", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: []string{"-brand"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal("Samsung", list.Devices[0].Brand)
	s.Require().Equal("Google", list.Devices[1].Brand)
	s.Require().Equal("Apple", list.Devices[2].Brand)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_SortByStateDescending() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("Device 1", "Brand", model.StateAvailable),
		model.NewDevice("Device 2", "Brand", model.StateInUse),
		model.NewDevice("Device 3", "Brand", model.StateInactive),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: []string{"-state"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal(model.StateInactive, list.Devices[0].State)
	s.Require().Equal(model.StateInUse, list.Devices[1].State)
	s.Require().Equal(model.StateAvailable, list.Devices[2].State)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_SortByUpdatedAtDescending() {
	ctx := s.T().Context()

	now := time.Now().UTC()
	devices := []*model.Device{
		{
			ID:        model.NewDeviceID(),
			Name:      "Old Device",
			Brand:     "Brand",
			State:     model.StateAvailable,
			CreatedAt: now.Add(-3 * time.Hour),
			UpdatedAt: now.Add(-3 * time.Hour),
		},
		{
			ID:        model.NewDeviceID(),
			Name:      "New Device",
			Brand:     "Brand",
			State:     model.StateAvailable,
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:        model.NewDeviceID(),
			Name:      "Middle Device",
			Brand:     "Brand",
			State:     model.StateAvailable,
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now.Add(-2 * time.Hour),
		},
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: []string{"-updatedAt"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal("New Device", list.Devices[0].Name)
	s.Require().Equal("Middle Device", list.Devices[1].Name)
	s.Require().Equal("Old Device", list.Devices[2].Name)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_InvalidSortFieldFallsBackToCreatedAt() {
	ctx := s.T().Context()

	now := time.Now().UTC()
	devices := []*model.Device{
		{
			ID:        model.NewDeviceID(),
			Name:      "Third",
			Brand:     "Brand",
			State:     model.StateAvailable,
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now,
		},
		{
			ID:        model.NewDeviceID(),
			Name:      "First",
			Brand:     "Brand",
			State:     model.StateAvailable,
			CreatedAt: now.Add(-3 * time.Hour),
			UpdatedAt: now,
		},
		{
			ID:        model.NewDeviceID(),
			Name:      "Second",
			Brand:     "Brand",
			State:     model.StateAvailable,
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now,
		},
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: []string{"invalidField"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
	s.Require().Equal("First", list.Devices[0].Name)
	s.Require().Equal("Second", list.Devices[1].Name)
	s.Require().Equal("Third", list.Devices[2].Name)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_EmptyBrandsFilterIsIgnored() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone", "Apple", model.StateAvailable),
		model.NewDevice("Galaxy", "Samsung", model.StateAvailable),
		model.NewDevice("Pixel", "Google", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Brands: []string{},
		Page:   1,
		Size:   20,
		Sort:   []string{"-createdAt"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 3)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_BrandFilterIsCaseSensitive() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone", "Apple", model.StateAvailable),
		model.NewDevice("MacBook", "apple", model.StateAvailable),
		model.NewDevice("iPad", "APPLE", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Brands: []string{"Apple"},
		Page:   1,
		Size:   20,
		Sort:   []string{"-createdAt"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 1)
	s.Require().Equal("Apple", list.Devices[0].Brand)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_CombinedFiltersAndSort() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone 15", "Apple", model.StateAvailable),
		model.NewDevice("iPhone 14", "Apple", model.StateAvailable),
		model.NewDevice("MacBook", "Apple", model.StateInUse),
		model.NewDevice("Galaxy", "Samsung", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	filter := model.DeviceFilter{
		Brands: []string{"Apple"},
		States: []model.State{model.StateAvailable},
		Page:   1,
		Size:   20,
		Sort:   []string{"name"},
	}

	list, err := s.repo.List(ctx, filter)

	s.Require().NoError(err)
	s.Require().Len(list.Devices, 2)
	s.Require().Equal("iPhone 14", list.Devices[0].Name)
	s.Require().Equal("iPhone 15", list.Devices[1].Name)
	for _, device := range list.Devices {
		s.Require().Equal("Apple", device.Brand)
		s.Require().Equal(model.StateAvailable, device.State)
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestUpdate_Success() {
	ctx := s.T().Context()

	device := model.NewDevice("Original", "Original Brand", model.StateAvailable)
	s.seedDevice(ctx, device)

	device.Name = "Updated"
	device.Brand = "Updated Brand"
	device.State = model.StateInUse
	device.UpdatedAt = time.Now().UTC()

	err := s.repo.Update(ctx, device)

	s.Require().NoError(err)

	retrieved, err := s.repo.FetchByID(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().Equal("Updated", retrieved.Name)
	s.Require().Equal("Updated Brand", retrieved.Brand)
	s.Require().Equal(model.StateInUse, retrieved.State)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestUpdate_NotFound() {
	ctx := s.T().Context()

	nonExistentDevice := model.NewDevice("Test", "Brand", model.StateAvailable)

	err := s.repo.Update(ctx, nonExistentDevice)

	s.Require().Error(err)
	s.Require().ErrorIs(err, model.ErrDeviceNotFound)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestUpdate_StateTransition() {
	ctx := s.T().Context()

	device := model.NewDevice("Device", "Brand", model.StateAvailable)
	s.seedDevice(ctx, device)

	transitions := []model.State{model.StateInUse, model.StateInactive, model.StateAvailable}

	for _, newState := range transitions {
		device.State = newState
		device.UpdatedAt = time.Now().UTC()

		err := s.repo.Update(ctx, device)
		s.Require().NoError(err)

		retrieved, err := s.repo.FetchByID(ctx, device.ID)
		s.Require().NoError(err)
		s.Require().Equal(newState, retrieved.State)
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestDelete_Success() {
	ctx := s.T().Context()

	device := model.NewDevice("To Delete", "Brand", model.StateAvailable)
	s.seedDevice(ctx, device)

	err := s.repo.Delete(ctx, device.ID)

	s.Require().NoError(err)

	retrieved, err := s.repo.FetchByID(ctx, device.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, model.ErrDeviceNotFound)
	s.Require().Nil(retrieved)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestDelete_NotFound() {
	ctx := s.T().Context()

	nonExistentID := model.NewDeviceID()

	err := s.repo.Delete(ctx, nonExistentID)

	s.Require().Error(err)
	s.Require().ErrorIs(err, model.ErrDeviceNotFound)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestPing_Success() {
	ctx := s.T().Context()

	err := s.repo.Ping(ctx)

	s.Require().NoError(err)
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_MultipleBrandsFilter() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone 15", "Apple", model.StateAvailable),
		model.NewDevice("iPhone 14", "Apple", model.StateInUse),
		model.NewDevice("Galaxy S24", "Samsung", model.StateAvailable),
		model.NewDevice("Galaxy S23", "Samsung", model.StateInactive),
		model.NewDevice("Pixel 8", "Google", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	cases := []struct {
		name          string
		brands        []string
		expectedCount int
	}{
		{
			name:          "Apple and Samsung",
			brands:        []string{"Apple", "Samsung"},
			expectedCount: 4,
		},
		{
			name:          "Apple and Google",
			brands:        []string{"Apple", "Google"},
			expectedCount: 3,
		},
		{
			name:          "Samsung and Google",
			brands:        []string{"Samsung", "Google"},
			expectedCount: 3,
		},
		{
			name:          "all brands",
			brands:        []string{"Apple", "Samsung", "Google"},
			expectedCount: 5,
		},
		{
			name:          "single brand backward compatible",
			brands:        []string{"Apple"},
			expectedCount: 2,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			filter := model.DeviceFilter{
				Brands: tc.brands,
				Page:   1,
				Size:   20,
				Sort:   []string{"-createdAt"},
			}

			list, err := s.repo.List(ctx, filter)

			s.Require().NoError(err)
			s.Require().Len(list.Devices, tc.expectedCount)
		})
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_MultipleStatesFilter() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone 15", "Apple", model.StateAvailable),
		model.NewDevice("iPhone 14", "Apple", model.StateInUse),
		model.NewDevice("Galaxy S24", "Samsung", model.StateAvailable),
		model.NewDevice("Galaxy S23", "Samsung", model.StateInactive),
		model.NewDevice("Pixel 8", "Google", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	cases := []struct {
		name          string
		states        []model.State
		expectedCount int
	}{
		{
			name:          "available and in-use",
			states:        []model.State{model.StateAvailable, model.StateInUse},
			expectedCount: 4,
		},
		{
			name:          "available and inactive",
			states:        []model.State{model.StateAvailable, model.StateInactive},
			expectedCount: 4,
		},
		{
			name:          "in-use and inactive",
			states:        []model.State{model.StateInUse, model.StateInactive},
			expectedCount: 2,
		},
		{
			name:          "all states",
			states:        []model.State{model.StateAvailable, model.StateInUse, model.StateInactive},
			expectedCount: 5,
		},
		{
			name:          "single state backward compatible",
			states:        []model.State{model.StateAvailable},
			expectedCount: 3,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			filter := model.DeviceFilter{
				States: tc.states,
				Page:   1,
				Size:   20,
				Sort:   []string{"-createdAt"},
			}

			list, err := s.repo.List(ctx, filter)

			s.Require().NoError(err)
			s.Require().Len(list.Devices, tc.expectedCount)
		})
	}
}

func (s *DevicesRepositoryIntegrationTestSuite) TestList_MultiValueCombinedFilters() {
	ctx := s.T().Context()

	devices := []*model.Device{
		model.NewDevice("iPhone 15", "Apple", model.StateAvailable),
		model.NewDevice("iPhone 14", "Apple", model.StateInUse),
		model.NewDevice("Galaxy S24", "Samsung", model.StateAvailable),
		model.NewDevice("Galaxy S23", "Samsung", model.StateInactive),
		model.NewDevice("Pixel 8", "Google", model.StateAvailable),
	}
	s.seedDevices(ctx, devices)

	cases := []struct {
		name          string
		brands        []string
		states        []model.State
		expectedCount int
	}{
		{
			name:          "Apple or Samsung AND available",
			brands:        []string{"Apple", "Samsung"},
			states:        []model.State{model.StateAvailable},
			expectedCount: 2,
		},
		{
			name:          "Apple AND available or in-use",
			brands:        []string{"Apple"},
			states:        []model.State{model.StateAvailable, model.StateInUse},
			expectedCount: 2,
		},
		{
			name:          "Apple or Samsung AND available or inactive",
			brands:        []string{"Apple", "Samsung"},
			states:        []model.State{model.StateAvailable, model.StateInactive},
			expectedCount: 3,
		},
		{
			name:          "Google AND in-use",
			brands:        []string{"Google"},
			states:        []model.State{model.StateInUse},
			expectedCount: 0,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			filter := model.DeviceFilter{
				Brands: tc.brands,
				States: tc.states,
				Page:   1,
				Size:   20,
				Sort:   []string{"-createdAt"},
			}

			list, err := s.repo.List(ctx, filter)

			s.Require().NoError(err)
			s.Require().Len(list.Devices, tc.expectedCount)
		})
	}
}

