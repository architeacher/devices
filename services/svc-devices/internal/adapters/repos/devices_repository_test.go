package repos_test

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func ptrString(s string) *string {
	return &s
}

func ptrState(s model.State) *model.State {
	return &s
}

func runRepoTest(
	t *testing.T,
	setupMock func(pgxmock.PgxPoolIface),
	testFn func(*testing.T, *repos.DevicesRepository),
) {
	t.Helper()
	t.Parallel()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	setupMock(mock)

	repo := repos.NewDevicesRepository(mock)
	testFn(t, repo)

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
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE id = $1`,
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
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE id = $1`,
				)).
					WithArgs(testID.String()).
					WillReturnError(pgx.ErrNoRows)
			},
			expectError: true,
			expectedErr: model.ErrDeviceNotFound,
		},
		{
			name:     "database error returns wrapped error",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE id = $1`,
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
				device, err := repo.GetByID(t.Context(), tc.deviceID)

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
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device 1", "Brand A", "available", now, now).
					AddRow(model.NewDeviceID().String(), "Device 2", "Brand B", "in-use", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
			name: "list with brand filter",
			filter: model.DeviceFilter{
				Brand: ptrString("Apple"),
				Page:  1,
				Size:  10,
				Sort:  "-createdAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "iPhone", "Apple", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE brand = $1 ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("Apple").
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE brand = $1`,
				)).
					WithArgs("Apple").
					WillReturnRows(countRows)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "list with state filter",
			filter: model.DeviceFilter{
				State: ptrState(model.StateInUse),
				Page:  1,
				Size:  10,
				Sort:  "-createdAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "in-use", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE state = $1 ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("in-use").
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE state = $1`,
				)).
					WithArgs("in-use").
					WillReturnRows(countRows)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "list with combined filters",
			filter: model.DeviceFilter{
				Brand: ptrString("Apple"),
				State: ptrState(model.StateAvailable),
				Page:  1,
				Size:  10,
				Sort:  "-createdAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "iPhone", "Apple", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE brand = $1 AND state = $2 ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("Apple", "available").
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(1))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE brand = $1 AND state = $2`,
				)).
					WithArgs("Apple", "available").
					WillReturnRows(countRows)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "list with ascending sort by name",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 10,
				Sort: "name",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Alpha", "Brand", "available", now, now).
					AddRow(model.NewDeviceID().String(), "Bravo", "Brand", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY name ASC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "-name",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Zulu", "Brand", "available", now, now).
					AddRow(model.NewDeviceID().String(), "Alpha", "Brand", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY name DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "-brand",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device", "Samsung", "available", now, now).
					AddRow(model.NewDeviceID().String(), "Device", "Apple", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY brand DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "-state",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "inactive", now, now).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY state DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "updatedAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldTime := now.Add(-2 * time.Hour)
				newTime := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Old Device", "Brand", "available", now, oldTime).
					AddRow(model.NewDeviceID().String(), "New Device", "Brand", "available", now, newTime)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY updated_at ASC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "-updatedAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldTime := now.Add(-2 * time.Hour)
				newTime := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "New Device", "Brand", "available", now, newTime).
					AddRow(model.NewDeviceID().String(), "Old Device", "Brand", "available", now, oldTime)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY updated_at DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "invalidField",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldCreated := now.Add(-2 * time.Hour)
				newCreated := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "First", "Brand", "available", oldCreated, now).
					AddRow(model.NewDeviceID().String(), "Second", "Brand", "available", newCreated, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at ASC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "-invalidField",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				oldCreated := now.Add(-2 * time.Hour)
				newCreated := now.Add(-1 * time.Hour)
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Second", "Brand", "available", newCreated, now).
					AddRow(model.NewDeviceID().String(), "First", "Brand", "available", oldCreated, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
			},
			expectError:   false,
			expectedCount: 2,
			validateList: func(t *testing.T, list *model.DeviceList) {
				require.Equal(t, "Second", list.Devices[0].Name)
				require.Equal(t, "First", list.Devices[1].Name)
			},
		},
		{
			name: "list with empty string brand filter is ignored",
			filter: model.DeviceFilter{
				Brand: ptrString(""),
				Page:  1,
				Size:  10,
				Sort:  "-createdAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device 1", "Apple", "available", now, now).
					AddRow(model.NewDeviceID().String(), "Device 2", "Samsung", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Sort: "-createdAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device 11", "Brand", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at DESC LIMIT 10 OFFSET 10`,
				)).
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(25))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(countRows)
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
				Brand: ptrString("NonExistent"),
				Page:  1,
				Size:  10,
				Sort:  "-createdAt",
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"})
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices WHERE brand = $1 ORDER BY created_at DESC LIMIT 10 OFFSET 0`,
				)).
					WithArgs("NonExistent").
					WillReturnRows(rows)

				countRows := pgxmock.NewRows([]string{"count"}).AddRow(uint(0))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE brand = $1`,
				)).
					WithArgs("NonExistent").
					WillReturnRows(countRows)
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
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WillReturnError(errors.New("connection error"))
			},
			expectError: true,
		},
		{
			name:   "count query error returns error",
			filter: model.DefaultDeviceFilter(),
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"id", "name", "brand", "state", "created_at", "updated_at"}).
					AddRow(model.NewDeviceID().String(), "Device", "Brand", "available", now, now)
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT id, name, brand, state, created_at, updated_at FROM devices ORDER BY created_at DESC LIMIT 20 OFFSET 0`,
				)).
					WillReturnRows(rows)

				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
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

func TestDevicesRepository_Exists(t *testing.T) {
	t.Parallel()

	testID := model.NewDeviceID()

	cases := []struct {
		name           string
		deviceID       model.DeviceID
		setupMock      func(mock pgxmock.PgxPoolIface)
		expectError    bool
		expectedErr    error
		expectedExists bool
	}{
		{
			name:     "device exists returns true",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"exists"}).AddRow(true)
				mock.ExpectQuery(`SELECT EXISTS\( SELECT 1 FROM devices WHERE id = \$1 \)`).
					WithArgs(testID.String()).
					WillReturnRows(rows)
			},
			expectError:    false,
			expectedExists: true,
		},
		{
			name:     "device does not exist returns false",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"exists"}).AddRow(false)
				mock.ExpectQuery(`SELECT EXISTS\( SELECT 1 FROM devices WHERE id = \$1 \)`).
					WithArgs(testID.String()).
					WillReturnRows(rows)
			},
			expectError:    false,
			expectedExists: false,
		},
		{
			name:     "database error returns wrapped ErrDatabaseQuery",
			deviceID: testID,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery(`SELECT EXISTS\( SELECT 1 FROM devices WHERE id = \$1 \)`).
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
				exists, err := repo.Exists(t.Context(), tc.deviceID)

				if tc.expectError {
					require.Error(t, err)
					if tc.expectedErr != nil {
						require.ErrorIs(t, err, tc.expectedErr)
					}
					require.False(t, exists)

					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.expectedExists, exists)
			})
		})
	}
}

func TestDevicesRepository_Count(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		filter        model.DeviceFilter
		setupMock     func(mock pgxmock.PgxPoolIface)
		expectError   bool
		expectedErr   error
		expectedCount uint
	}{
		{
			name:   "count all devices",
			filter: model.DeviceFilter{},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"count"}).AddRow(uint(10))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 10,
		},
		{
			name: "count with brand filter",
			filter: model.DeviceFilter{
				Brand: ptrString("Apple"),
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"count"}).AddRow(uint(5))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE brand = $1`,
				)).
					WithArgs("Apple").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 5,
		},
		{
			name: "count with state filter",
			filter: model.DeviceFilter{
				State: ptrState(model.StateInUse),
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"count"}).AddRow(uint(3))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE state = $1`,
				)).
					WithArgs("in-use").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name: "count with combined filters",
			filter: model.DeviceFilter{
				Brand: ptrString("Apple"),
				State: ptrState(model.StateAvailable),
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"count"}).AddRow(uint(2))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices WHERE brand = $1 AND state = $2`,
				)).
					WithArgs("Apple", "available").
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:   "count empty result",
			filter: model.DeviceFilter{},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"count"}).AddRow(uint(0))
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnRows(rows)
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:   "database error returns wrapped ErrDatabaseQuery",
			filter: model.DeviceFilter{},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery(regexp.QuoteMeta(
					`SELECT COUNT(*) FROM devices`,
				)).
					WillReturnError(errors.New("connection error"))
			},
			expectError: true,
			expectedErr: model.ErrDatabaseQuery,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRepoTest(t, tc.setupMock, func(t *testing.T, repo *repos.DevicesRepository) {
				count, err := repo.Count(t.Context(), tc.filter)

				if tc.expectError {
					require.Error(t, err)
					if tc.expectedErr != nil {
						require.ErrorIs(t, err, tc.expectedErr)
					}
					require.Equal(t, uint(0), count)

					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.expectedCount, count)
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
