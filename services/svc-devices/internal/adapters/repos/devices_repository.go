package repos

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const devicesTable = "devices"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type (
	// PoolOps defines the interface for database operations.
	// This allows injecting mock implementations for testing.
	PoolOps interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
		Ping(ctx context.Context) error
	}

	// DevicesRepository handles device persistence operations.
	DevicesRepository struct {
		pool       PoolOps
		scanner    Scanner
		logger     logger.Logger
		translator *CriteriaTranslator
	}

	deviceRow struct {
		ID        string    `db:"id"`
		Name      string    `db:"name"`
		Brand     string    `db:"brand"`
		State     string    `db:"state"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}

	deviceRowWithCount struct {
		deviceRow
		TotalCount uint `db:"total_count"`
	}
)

// NewDevicesRepository creates a new DevicesRepository with the given dependencies.
func NewDevicesRepository(
	pool PoolOps,
	scanner Scanner,
	translator *CriteriaTranslator,
	log logger.Logger,
) *DevicesRepository {
	return &DevicesRepository{
		pool:       pool,
		scanner:    scanner,
		translator: translator,
		logger:     log,
	}
}

func (r *DevicesRepository) Create(ctx context.Context, device *model.Device) error {
	query, args, err := psql.Insert(devicesTable).
		Columns("id", "name", "brand", "state", "created_at", "updated_at").
		Values(
			device.ID.String(),
			device.Name,
			device.Brand,
			device.State.String(),
			device.CreatedAt,
			device.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		if isDuplicateKeyError(err) {
			return model.ErrDuplicateDevice
		}

		return fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	return nil
}

func (r *DevicesRepository) FetchByID(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	return r.findByCriteria(
		ctx,
		sq.Eq{"id": id.String()},
		fmt.Sprintf("device with ID %s not found", id.String()),
	)
}

func (r *DevicesRepository) List(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	criteria := model.FromDeviceFilter(filter)

	selectBuilder := psql.Select(
		"id", "name", "brand", "state", "created_at", "updated_at",
		"COUNT(*) OVER() as total_count",
	).From(devicesTable)

	selectBuilder = r.translator.ApplyToSelect(selectBuilder, criteria)

	devices, totalItems, err := r.queryDevicesWithCount(ctx, selectBuilder)
	if err != nil {
		return nil, err
	}

	totalPages := totalItems / criteria.Size()
	if totalItems%criteria.Size() != 0 {
		totalPages++
	}

	pagination := model.Pagination{
		Page:        criteria.Page(),
		Size:        criteria.Size(),
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasNext:     criteria.Page() < totalPages,
		HasPrevious: criteria.Page() > 1,
	}

	sortField := r.getPrimarySortField(filter)
	pagination = r.generateCursors(devices, pagination, sortField)

	return &model.DeviceList{
		Devices:    devices,
		Pagination: pagination,
		Filters:    filter,
	}, nil
}

func (r *DevicesRepository) getPrimarySortField(filter model.DeviceFilter) string {
	if len(filter.Sort) > 0 {
		return filter.Sort[0]
	}

	return "-createdAt"
}

func (r *DevicesRepository) generateCursors(
	devices []*model.Device,
	pagination model.Pagination,
	sortField string,
) model.Pagination {
	if len(devices) == 0 {
		return pagination
	}

	lastDevice := devices[len(devices)-1]
	if pagination.HasNext {
		cursor := model.NewCursorFromDevice(lastDevice, sortField, model.CursorDirectionNext)
		if encoded, err := model.EncodeCursor(cursor); err == nil {
			pagination.NextCursor = encoded
		}
	}

	firstDevice := devices[0]
	if pagination.HasPrevious {
		cursor := model.NewCursorFromDevice(firstDevice, sortField, model.CursorDirectionPrev)
		if encoded, err := model.EncodeCursor(cursor); err == nil {
			pagination.PreviousCursor = encoded
		}
	}

	return pagination
}

func (r *DevicesRepository) Update(ctx context.Context, device *model.Device) error {
	return r.updateByCriteria(
		ctx,
		psql.Update(devicesTable).
			Set("name", device.Name).
			Set("brand", device.Brand).
			Set("state", device.State.String()).
			Set("updated_at", device.UpdatedAt).
			Where(sq.Eq{"id": device.ID.String()}),
		"failed to update device",
	)
}

func (r *DevicesRepository) Delete(ctx context.Context, id model.DeviceID) error {
	query, args, err := psql.Delete(devicesTable).
		Where(sq.Eq{"id": id.String()}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete query: %w", err)
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	if result.RowsAffected() == 0 {
		return model.ErrDeviceNotFound
	}

	return nil
}

func (r *DevicesRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *DevicesRepository) findByCriteria(
	ctx context.Context,
	criteria sq.Sqlizer,
	errorContext string,
) (*model.Device, error) {
	query, args, err := psql.Select("id", "name", "brand", "state", "created_at", "updated_at").
		From(devicesTable).
		Where(criteria).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}
	defer rows.Close()

	var row deviceRow
	if err := r.scanner.ScanOne(&row, rows); err != nil {
		if r.scanner.IsNotFound(err) {
			return nil, model.ErrDeviceNotFound
		}

		return nil, fmt.Errorf("%s: %w", errorContext, err)
	}

	return r.convertRowToDevice(row)
}

func (r *DevicesRepository) updateByCriteria(
	ctx context.Context,
	updateBuilder sq.UpdateBuilder,
	errorContext string,
) error {
	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", errorContext, err)
	}

	if result.RowsAffected() == 0 {
		return model.ErrDeviceNotFound
	}

	return nil
}

func (r *DevicesRepository) queryDevices(ctx context.Context, builder sq.SelectBuilder) ([]*model.Device, error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}
	defer rows.Close()

	var deviceRows []deviceRow
	if err := r.scanner.ScanAll(&deviceRows, rows); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	devices := make([]*model.Device, 0, len(deviceRows))
	for index := range deviceRows {
		device, err := r.convertRowToDevice(deviceRows[index])
		if err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (r *DevicesRepository) queryDevicesWithCount(ctx context.Context, builder sq.SelectBuilder) ([]*model.Device, uint, error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}
	defer rows.Close()

	var deviceRows []deviceRowWithCount
	if err := r.scanner.ScanAll(&deviceRows, rows); err != nil {
		return nil, 0, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	if len(deviceRows) == 0 {
		return []*model.Device{}, 0, nil
	}

	totalCount := deviceRows[0].TotalCount
	devices := make([]*model.Device, 0, len(deviceRows))

	for index := range deviceRows {
		device, err := r.convertRowToDevice(deviceRows[index].deviceRow)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
		}
		devices = append(devices, device)
	}

	return devices, totalCount, nil
}

func (r *DevicesRepository) convertRowToDevice(row deviceRow) (*model.Device, error) {
	id, err := model.ParseDeviceID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse device ID: %w", err)
	}

	state, err := model.ParseState(row.State)
	if err != nil {
		return nil, fmt.Errorf("failed to parse device state: %w", err)
	}

	return &model.Device{
		ID:        id,
		Name:      row.Name,
		Brand:     row.Brand,
		State:     state,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func isDuplicateKeyError(err error) bool {
	return err != nil && (errors.Is(err, pgx.ErrNoRows) == false) &&
		(err.Error() != "" && len(err.Error()) > 0 &&
			(contains(err.Error(), "duplicate key") || contains(err.Error(), "unique constraint")))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
