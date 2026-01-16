package repos_test

import (
	"bytes"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func runRepoTest(
	t *testing.T,
	setupMock func(pgxmock.PgxPoolIface),
	testFn func(*testing.T, *repos.DevicesRepository),
) {
	runRepoTestWithLogger(t, setupMock, func(t *testing.T, repo *repos.DevicesRepository, _ *bytes.Buffer) {
		testFn(t, repo)
	})
}

func runRepoTestWithLogger(
	t *testing.T,
	setupMock func(pgxmock.PgxPoolIface),
	testFn func(*testing.T, *repos.DevicesRepository, *bytes.Buffer),
) {
	t.Helper()
	t.Parallel()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	setupMock(mock)

	logBuffer := &bytes.Buffer{}
	log := logger.NewBufferedTestLogger(logBuffer)
	repo := repos.NewDevicesRepository(mock, repos.NewPgxScanner(), repos.NewCriteriaTranslator(&log), log)
	testFn(t, repo, logBuffer)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDevicesRepository_Create(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		device      *model.Device
		setupMock   func(mock pgxmock.PgxPoolIface, device *model.Device)
		expectError bool
		expectedErr error
	}{
		{
			name:   "successfully create device",
			device: model.NewDevice("Test Device", "Test Brand", model.StateAvailable),
			setupMock: func(mock pgxmock.PgxPoolIface, device *model.Device) {
				mock.ExpectExec(regexp.QuoteMeta(
					`INSERT INTO devices (id,name,brand,state,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
				)).
					WithArgs(
						device.ID.String(),
						device.Name,
						device.Brand,
						device.State.String(),
						device.CreatedAt,
						device.UpdatedAt,
					).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
			},
			expectError: false,
		},
		{
			name:   "duplicate key error returns ErrDuplicateDevice",
			device: model.NewDevice("Duplicate", "Brand", model.StateAvailable),
			setupMock: func(mock pgxmock.PgxPoolIface, device *model.Device) {
				mock.ExpectExec(regexp.QuoteMeta(
					`INSERT INTO devices (id,name,brand,state,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
				)).
					WithArgs(
						device.ID.String(),
						device.Name,
						device.Brand,
						device.State.String(),
						device.CreatedAt,
						device.UpdatedAt,
					).
					WillReturnError(errors.New("duplicate key value violates unique constraint"))
			},
			expectError: true,
			expectedErr: model.ErrDuplicateDevice,
		},
		{
			name:   "unique constraint violation returns ErrDuplicateDevice",
			device: model.NewDevice("Duplicate", "Brand", model.StateAvailable),
			setupMock: func(mock pgxmock.PgxPoolIface, device *model.Device) {
				mock.ExpectExec(regexp.QuoteMeta(
					`INSERT INTO devices (id,name,brand,state,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
				)).
					WithArgs(
						device.ID.String(),
						device.Name,
						device.Brand,
						device.State.String(),
						device.CreatedAt,
						device.UpdatedAt,
					).
					WillReturnError(errors.New("unique constraint violation"))
			},
			expectError: true,
			expectedErr: model.ErrDuplicateDevice,
		},
		{
			name:   "database error returns wrapped ErrDatabaseQuery",
			device: model.NewDevice("Error Device", "Brand", model.StateAvailable),
			setupMock: func(mock pgxmock.PgxPoolIface, device *model.Device) {
				mock.ExpectExec(regexp.QuoteMeta(
					`INSERT INTO devices (id,name,brand,state,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
				)).
					WithArgs(
						device.ID.String(),
						device.Name,
						device.Brand,
						device.State.String(),
						device.CreatedAt,
						device.UpdatedAt,
					).
					WillReturnError(errors.New("connection refused"))
			},
			expectError: true,
			expectedErr: model.ErrDatabaseQuery,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, func(mock pgxmock.PgxPoolIface) {
				tc.setupMock(mock, tc.device)
			}, func(t *testing.T, repo *repos.DevicesRepository) {
				err := repo.Create(t.Context(), tc.device)

				if tc.expectError {
					require.Error(t, err)
					if tc.expectedErr != nil {
						require.ErrorIs(t, err, tc.expectedErr)
					}

					return
				}
				require.NoError(t, err)
			})
		})
	}
}

func TestDevicesRepository_GetByID(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	testID := model.NewDeviceID()

	cases := []struct {
		name           string
		deviceID       model.DeviceID
		setupMock      func(mock pgxmock.PgxPoolIface)
		expectError    bool
		expectedErr    error
		expectedDevice *model.Device
	}{
		{
			name:     "successfully get device",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(testID.String(), "Test Device", "Test Brand", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE id = $1 LIMIT 1`,
				)).
					WithArgs(testID.String()).
					WillReturnRows(rows)
			},
			expectError: false,
			expectedDevice: &model.Device{
				ID:        testID,
				Name:      "Test Device",
				Brand:     "Test Brand",
				State:     model.StateAvailable,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		{
			name:     "device not found returns ErrDeviceNotFound",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				emptyRows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"})
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE id = $1 LIMIT 1`,
				)).
					WithArgs(testID.String()).
					WillReturnRows(emptyRows)
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
		{
			name:     "database error returns wrapped error",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE id = $1 LIMIT 1`,
				)).
					WithArgs(testID.String()).
					WillReturnError(errors.New("connection error"))
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				device, err := repo.FetchByID(t.Context(), tc.deviceID)

				if tc.expectError {
					require.Error(t, err)
					if tc.expectedErr != nil {
						require.ErrorIs(t, err, tc.expectedErr)
					}
					require.Nil(t, device)

					return
				}
				require.NoError(t, err)
				require.NotNil(t, device)
				require.Equal(t, tc.expectedDevice.ID, device.ID)
				require.Equal(t, tc.expectedDevice.Name, device.Name)
				require.Equal(t, tc.expectedDevice.Brand, device.Brand)
				require.Equal(t, tc.expectedDevice.State, device.State)
			})
		})
	}
}

func TestDevicesRepository_List(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	cases := []struct {
		name          string
		filter        model.DeviceFilter
		setupMock     func(mock pgxmock.PgxPoolIface)
		expectError   bool
		expectedCount int
		validateList  func(*testing.T, *model.DeviceList)
	}{
		{
			name:   "list all devices with default pagination",
			filter: model.DefaultDeviceFilter(),
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device 1", "Brand A", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Device 2", "Brand B", "in-use", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, uint(1), list.Pagination.Page)
				require.Equal(t, uint(20), list.Pagination.Size)
				require.Equal(t, uint(2), list.Pagination.TotalItems)
				require.Equal(t, uint(1), list.Pagination.TotalPages)
				require.False(t, list.Pagination.HasNext)
				require.False(t, list.Pagination.HasPrevious)
			},
		},
		{
			name: "list with single brand filter",
			filter: model.DeviceFilter{
				Brands: []string{"Apple"},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone", "Apple", "available", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE brand IN ($1) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("Apple").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "list with single state filter",
			filter: model.DeviceFilter{
				States: []model.State{model.StateInUse},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "in-use", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE state IN ($1) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("in-use").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "list with combined single-value filters",
			filter: model.DeviceFilter{
				Brands: []string{"Apple"},
				States: []model.State{model.StateAvailable},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone", "Apple", "available", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE (brand IN ($1) AND state IN ($2)) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("Apple", "available").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "list with multiple brands filter (OR within field)",
			filter: model.DeviceFilter{
				Brands: []string{"Apple", "Samsung"},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone", "Apple", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Galaxy", "Samsung", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE brand IN ($1,$2) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("Apple", "Samsung").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				brands := make(map[string]bool)
				for _, d := range list.Devices {
					brands[d.Brand] = true
				}
				require.True(t, brands["Apple"])
				require.True(t, brands["Samsung"])
			},
		},
		{
			name: "list with multiple states filter (OR within field)",
			filter: model.DeviceFilter{
				States: []model.State{model.StateAvailable, model.StateInactive},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device 1", "Brand", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Device 2", "Brand", "inactive", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE state IN ($1,$2) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("available", "inactive").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				states := make(map[model.State]bool)
				for _, d := range list.Devices {
					states[d.State] = true
				}
				require.True(t, states[model.StateAvailable])
				require.True(t, states[model.StateInactive])
			},
		},
		{
			name: "list with combined multi-value filters (AND between fields)",
			filter: model.DeviceFilter{
				Brands: []string{"Apple", "Samsung"},
				States: []model.State{model.StateAvailable},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone", "Apple", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Galaxy", "Samsung", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE (brand IN ($1,$2) AND state IN ($3)) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("Apple", "Samsung", "available").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				for _, d := range list.Devices {
					require.Equal(t, model.StateAvailable, d.State)
					require.True(t, d.Brand == "Apple" || d.Brand == "Samsung")
				}
			},
		},
		{
			name: "list with ascending sort by name",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"name"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Alpha", "Brand", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Bravo", "Brand", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY name ASC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Alpha", list.Devices[0].Name)
				require.Equal(t, "Bravo", list.Devices[1].Name)
			},
		},
		{
			name: "list with descending sort by name",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"-name"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Zulu", "Brand", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Alpha", "Brand", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY name DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Zulu", list.Devices[0].Name)
				require.Equal(t, "Alpha", list.Devices[1].Name)
			},
		},
		{
			name: "list with descending sort by brand",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"-brand"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device", "Samsung", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Device", "Apple", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY brand DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Samsung", list.Devices[0].Brand)
				require.Equal(t, "Apple", list.Devices[1].Brand)
			},
		},
		{
			name: "list with descending sort by state",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"-state"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "inactive", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY state DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, model.StateInactive, list.Devices[0].State)
				require.Equal(t, model.StateAvailable, list.Devices[1].State)
			},
		},
		{
			name: "list with ascending sort by updatedAt",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"updatedAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldTime := now.Add(-2 * time.Hour)
				newTime := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Old Device", "Brand", "available", now, oldTime, uint(2)).
					AddRow(model.NewDeviceID().String(), "New Device", "Brand", "available", now, newTime, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY updated_at ASC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Old Device", list.Devices[0].Name)
				require.Equal(t, "New Device", list.Devices[1].Name)
			},
		},
		{
			name: "list with descending sort by updatedAt",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"-updatedAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldTime := now.Add(-2 * time.Hour)
				newTime := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "New Device", "Brand", "available", now, newTime, uint(2)).
					AddRow(model.NewDeviceID().String(), "Old Device", "Brand", "available", now, oldTime, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY updated_at DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "New Device", list.Devices[0].Name)
				require.Equal(t, "Old Device", list.Devices[1].Name)
			},
		},
		{
			name: "list with invalid sort field falls back to created_at",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"invalidField"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldCreated := now.Add(-2 * time.Hour)
				newCreated := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "First", "Brand", "available", oldCreated, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Second", "Brand", "available", newCreated, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at ASC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "First", list.Devices[0].Name)
				require.Equal(t, "Second", list.Devices[1].Name)
			},
		},
		{
			name: "list with invalid descending sort field falls back to created_at",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: []string{"-invalidField"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldCreated := now.Add(-2 * time.Hour)
				newCreated := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Second", "Brand", "available", newCreated, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "First", "Brand", "available", oldCreated, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Second", list.Devices[0].Name)
				require.Equal(t, "First", list.Devices[1].Name)
			},
		},
		{
			name: "list with empty brands slice is ignored",
			filter: model.DeviceFilter{
				Brands: []string{},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device 1", "Apple", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Device 2", "Samsung", "available", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Apple", list.Devices[0].Brand)
				require.Equal(t, "Samsung", list.Devices[1].Brand)
			},
		},
		{
			name: "list with pagination offset",
			filter: model.DeviceFilter{
				Page: 2,
				Size: 10,
				Sort: []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device 11", "Brand", "available", now, now, uint(25))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at DESC LIMIT 10 OFFSET 10`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, uint(2), list.Pagination.Page)
				require.Equal(t, uint(25), list.Pagination.TotalItems)
				require.Equal(t, uint(3), list.Pagination.TotalPages)
				require.True(t, list.Pagination.HasNext)
				require.True(t, list.Pagination.HasPrevious)
			},
		},
		{
			name: "list empty result",
			filter: model.DeviceFilter{
				Brands: []string{"NonExistent"},
				Page:   1,
				Size:   10,
				Sort:   []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"})
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE brand IN ($1) ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("NonExistent").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 0,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, uint(0), list.Pagination.TotalItems)
				require.Equal(t, uint(0), list.Pagination.TotalPages)
				require.False(t, list.Pagination.HasNext)
				require.False(t, list.Pagination.HasPrevious)
			},
		},
		{
			name:   "list query error returns error",
			filter: model.DefaultDeviceFilter(),
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WillReturnError(errors.New("connection error"))
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				list, err := repo.List(t.Context(), tc.filter)

				if tc.expectError {
					require.Error(t, err)
					require.Nil(t, list)

					return
				}
				require.NoError(t, err)
				require.NotNil(t, list)
				require.Len(t, list.Devices, tc.expectedCount)
				if tc.validateList != nil {
					tc.validateList(t, list)
				}
			})
		})
	}
}

func TestDevicesRepository_Update(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	testID := model.NewDeviceID()

	cases := []struct {
		name        string
		device      *model.Device
		setupMock   func(mock pgxmock.PgxPoolIface)
		expectError bool
		expectedErr error
	}{
		{
			name: "successfully update device",
			device: &model.Device{
				ID:        testID,
				Name:      "Updated Name",
				Brand:     "Updated Brand",
				State:     model.StateInUse,
				UpdatedAt: now,
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec(regexp.QuoteMeta(
					`UPDATE devices SET name = $1, brand = $2, state = $3, updated_at = $4 WHERE id = $5`,
				)).
					WithArgs("Updated Name", "Updated Brand", "in-use", now, testID.String()).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
			expectError: false,
		},
		{
			name: "update nonexistent device returns ErrDeviceNotFound",
			device: &model.Device{
				ID:        testID,
				Name:      "Updated Name",
				Brand:     "Updated Brand",
				State:     model.StateAvailable,
				UpdatedAt: now,
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec(regexp.QuoteMeta(
					`UPDATE devices SET name = $1, brand = $2, state = $3, updated_at = $4 WHERE id = $5`,
				)).
					WithArgs("Updated Name", "Updated Brand", "available", now, testID.String()).
					WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
		{
			name: "database error returns wrapped error",
			device: &model.Device{
				ID:        testID,
				Name:      "Updated Name",
				Brand:     "Updated Brand",
				State:     model.StateAvailable,
				UpdatedAt: now,
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec(regexp.QuoteMeta(
					`UPDATE devices SET name = $1, brand = $2, state = $3, updated_at = $4 WHERE id = $5`,
				)).
					WithArgs("Updated Name", "Updated Brand", "available", now, testID.String()).
					WillReturnError(errors.New("connection error"))
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				err := repo.Update(t.Context(), tc.device)

				if tc.expectError {
					require.Error(t, err)
					if tc.expectedErr != nil {
						require.ErrorIs(t, err, tc.expectedErr)
					}

					return
				}
				require.NoError(t, err)
			})
		})
	}
}

func TestDevicesRepository_Delete(t *testing.T) {
	t.Parallel()

	testID := model.NewDeviceID()

	cases := []struct {
		name        string
		deviceID    model.DeviceID
		setupMock   func(mock pgxmock.PgxPoolIface)
		expectError bool
		expectedErr error
	}{
		{
			name:     "successfully delete device",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec(regexp.QuoteMeta(
					`DELETE FROM devices WHERE id = $1`,
				)).
					WithArgs(testID.String()).
					WillReturnResult(pgxmock.NewResult("DELETE", 1))
			},
			expectError: false,
		},
		{
			name:     "delete nonexistent device returns ErrDeviceNotFound",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec(regexp.QuoteMeta(
					`DELETE FROM devices WHERE id = $1`,
				)).
					WithArgs(testID.String()).
					WillReturnResult(pgxmock.NewResult("DELETE", 0))
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
		{
			name:     "database error returns wrapped ErrDatabaseQuery",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec(regexp.QuoteMeta(
					`DELETE FROM devices WHERE id = $1`,
				)).
					WithArgs(testID.String()).
					WillReturnError(errors.New("connection error"))
			},
			expectError: true,
			expectedErr: model.ErrDatabaseQuery,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				err := repo.Delete(t.Context(), tc.deviceID)

				if tc.expectError {
					require.Error(t, err)
					if tc.expectedErr != nil {
						require.ErrorIs(t, err, tc.expectedErr)
					}

					return
				}
				require.NoError(t, err)
			})
		})
	}
}

func TestDevicesRepository_Ping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		setupMock   func(mock pgxmock.PgxPoolIface)
		expectError bool
	}{
		{
			name: "ping successful",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectPing()
			},
			expectError: false,
		},
		{
			name: "ping failed",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectPing().WillReturnError(errors.New("connection error"))
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				err := repo.Ping(t.Context())

				if tc.expectError {
					require.Error(t, err)

					return
				}
				require.NoError(t, err)
			})
		})
	}
}

func TestDevicesRepository_List_WithFullTextSearch(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	cases := []struct {
		name          string
		filter        model.DeviceFilter
		setupMock     func(mock pgxmock.PgxPoolIface)
		expectError   bool
		expectedCount int
		validateList  func(*testing.T, *model.DeviceList)
	}{
		{
			name: "search by exact name match",
			filter: model.DeviceFilter{
				Keyword: "iPhone",
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone 15 Pro", "Apple", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "iPhone 14", "Apple", "in-use", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE search_vector @@ plainto_tsquery('english', $1) ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WithArgs("iPhone").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				for _, d := range list.Devices {
					require.Contains(t, d.Name, "iPhone")
				}
			},
		},
		{
			name: "search by brand match",
			filter: model.DeviceFilter{
				Keyword: "Samsung",
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Galaxy S24", "Samsung", "available", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE search_vector @@ plainto_tsquery('english', $1) ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WithArgs("Samsung").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Samsung", list.Devices[0].Brand)
			},
		},
		{
			name: "search with no results",
			filter: model.DeviceFilter{
				Keyword: "nonexistent",
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"})
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE search_vector @@ plainto_tsquery('english', $1) ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WithArgs("nonexistent").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 0,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, uint(0), list.Pagination.TotalItems)
			},
		},
		{
			name: "search combined with state filter",
			filter: model.DeviceFilter{
				Keyword: "iPhone",
				States:  []model.State{model.StateAvailable},
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone 15 Pro", "Apple", "available", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE (search_vector @@ plainto_tsquery('english', $1) AND state IN ($2)) ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WithArgs("iPhone", "available").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, model.StateAvailable, list.Devices[0].State)
				require.Contains(t, list.Devices[0].Name, "iPhone")
			},
		},
		{
			name: "search combined with brand filter",
			filter: model.DeviceFilter{
				Keyword: "Pro",
				Brands:  []string{"Apple"},
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "iPhone 15 Pro", "Apple", "available", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE (search_vector @@ plainto_tsquery('english', $1) AND brand IN ($2)) ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WithArgs("Pro", "Apple").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Apple", list.Devices[0].Brand)
				require.Contains(t, list.Devices[0].Name, "Pro")
			},
		},
		{
			name: "search combined with all filters",
			filter: model.DeviceFilter{
				Keyword: "Galaxy",
				Brands:  []string{"Samsung"},
				States:  []model.State{model.StateAvailable},
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Galaxy S24 Ultra", "Samsung", "available", now, now, uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices WHERE (search_vector @@ plainto_tsquery('english', $1) AND brand IN ($2) AND state IN ($3)) ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WithArgs("Galaxy", "Samsung", "available").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 1,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Samsung", list.Devices[0].Brand)
				require.Equal(t, model.StateAvailable, list.Devices[0].State)
				require.Contains(t, list.Devices[0].Name, "Galaxy")
			},
		},
		{
			name: "empty search string is ignored",
			filter: model.DeviceFilter{
				Keyword: "",
				Page:    1,
				Size:    20,
				Sort:    []string{"-createdAt"},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device 1", "Brand A", "available", now, now, uint(2)).
					AddRow(model.NewDeviceID().String(), "Device 2", "Brand B", "in-use", now, now, uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at, COUNT(*) OVER() as total_count FROM devices ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				list, err := repo.List(t.Context(), tc.filter)

				if tc.expectError {
					require.Error(t, err)
					require.Nil(t, list)

					return
				}
				require.NoError(t, err)
				require.NotNil(t, list)
				require.Len(t, list.Devices, tc.expectedCount)
				if tc.validateList != nil {
					tc.validateList(t, list)
				}
			})
		})
	}
}

func TestDevicesRepository_List_LogsWarningForInvalidSortField(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	cases := []struct {
		name          string
		sortField     string
		expectedField string
	}{
		{
			name:          "invalid ascending sort field logs warning",
			sortField:     "invalidField",
			expectedField: "invalidField",
		},
		{
			name:          "invalid descending sort field logs warning",
			sortField:     "-unknownColumn",
			expectedField: "unknownColumn",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTestWithLogger(t, func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at", "total_count"}).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "available", now, now, uint(1))
				mock.ExpectQuery(`SELECT id, name, brand, state, created_at, updated_at, COUNT\(\*\) OVER\(\) as total_count FROM devices ORDER BY created_at`).
					WillReturnRows(rows)
			}, func(t *testing.T, repo *repos.DevicesRepository, logBuffer *bytes.Buffer) {
				filter := model.DeviceFilter{
					Page: 1,
					Size: 10,
					Sort: []string{tc.sortField},
				}

				_, err := repo.List(t.Context(), filter)
				require.NoError(t, err)

				logOutput := logBuffer.String()
				require.Contains(t, logOutput, "unknown sort field requested")
				require.Contains(t, logOutput, tc.expectedField)
				require.Contains(t, logOutput, "created_at")
				require.Contains(t, logOutput, "warn")
			})
		})
	}
}
