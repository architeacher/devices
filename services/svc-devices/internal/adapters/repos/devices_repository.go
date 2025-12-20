package repos

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
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

	DevicesRepository struct {
		pool PoolOps
	}

	deviceRow struct {
		ID        string    `db:"id"`
		Name      string    `db:"name"`
		Brand     string    `db:"brand"`
		State     string    `db:"state"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
)

func NewDevicesRepository(pool PoolOps) *DevicesRepository {
	return &DevicesRepository{pool: pool}
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

func (r *DevicesRepository) GetByID(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	return r.findByCriteria(
		ctx,
		sq.Eq{"id": id.String()},
		fmt.Sprintf("device with ID %s not found", id.String()),
	)
}

func (r *DevicesRepository) List(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	selectBuilder := psql.Select("id", "name", "brand", "state", "created_at", "updated_at").
		From(devicesTable)

	countBuilder := psql.Select("COUNT(*)").
		From(devicesTable)

	selectBuilder, countBuilder = r.applyFilters(selectBuilder, countBuilder, filter)

	selectBuilder = r.applyOrdering(selectBuilder, filter.Sort)

	offset := (filter.Page - 1) * filter.Size
	selectBuilder = selectBuilder.Limit(uint64(filter.Size)).Offset(uint64(offset))

	devices, err := r.queryDevices(ctx, selectBuilder)
	if err != nil {
		return nil, err
	}

	totalItems, err := r.countDevices(ctx, countBuilder)
	if err != nil {
		return nil, err
	}

	totalPages := totalItems / filter.Size
	if totalItems%filter.Size != 0 {
		totalPages++
	}

	return &model.DeviceList{
		Devices: devices,
		Pagination: model.Pagination{
			Page:        filter.Page,
			Size:        filter.Size,
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			HasNext:     filter.Page < totalPages,
			HasPrevious: filter.Page > 1,
		},
		Filters: filter,
	}, nil
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

func (r *DevicesRepository) Exists(ctx context.Context, id model.DeviceID) (bool, error) {
	query, args, err := psql.Select("1").
		Prefix("SELECT EXISTS(").
		From(devicesTable).
		Where(sq.Eq{"id": id.String()}).
		Suffix(")").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("failed to build exists query: %w", err)
	}

	var exists bool

	err = r.pool.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	return exists, nil
}

func (r *DevicesRepository) Count(ctx context.Context, filter model.DeviceFilter) (uint, error) {
	countBuilder := psql.Select("COUNT(*)").From(devicesTable)
	countBuilder = r.applyFilterConditions(countBuilder, filter)

	return r.countDevices(ctx, countBuilder)
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
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	row := r.pool.QueryRow(ctx, query, args...)

	device, err := r.scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrDeviceNotFound
		}

		return nil, fmt.Errorf("%s: %w", errorContext, err)
	}

	return device, nil
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

func (r *DevicesRepository) applyFilters(
	selectBuilder sq.SelectBuilder,
	countBuilder sq.SelectBuilder,
	filter model.DeviceFilter,
) (sq.SelectBuilder, sq.SelectBuilder) {
	selectBuilder = r.applyFilterConditions(selectBuilder, filter)
	countBuilder = r.applyFilterConditions(countBuilder, filter)

	return selectBuilder, countBuilder
}

func (r *DevicesRepository) applyFilterConditions(builder sq.SelectBuilder, filter model.DeviceFilter) sq.SelectBuilder {
	if filter.Brand != nil && *filter.Brand != "" {
		builder = builder.Where(sq.Eq{"brand": *filter.Brand})
	}

	if filter.State != nil {
		builder = builder.Where(sq.Eq{"state": filter.State.String()})
	}

	return builder
}

func (r *DevicesRepository) applyOrdering(builder sq.SelectBuilder, sort string) sq.SelectBuilder {
	if sort == "" {
		sort = "-createdAt"
	}

	direction := "ASC"
	field := sort

	if len(sort) > 0 && sort[0] == '-' {
		direction = "DESC"
		field = sort[1:]
	}

	columnMap := map[string]string{
		"createdAt": "created_at",
		"updatedAt": "updated_at",
		"name":      "name",
		"brand":     "brand",
		"state":     "state",
	}

	column, ok := columnMap[field]
	if !ok {
		column = "created_at"
	}

	return builder.OrderBy(fmt.Sprintf("%s %s", column, direction))
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

	var devices []*model.Device

	for rows.Next() {
		device, err := r.scanDeviceFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
		}

		devices = append(devices, device)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	return devices, nil
}

func (r *DevicesRepository) countDevices(ctx context.Context, builder sq.SelectBuilder) (uint, error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	var count uint

	err = r.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", model.ErrDatabaseQuery, err)
	}

	return count, nil
}

func (r *DevicesRepository) scanDevice(row pgx.Row) (*model.Device, error) {
	var deviceRow deviceRow

	err := row.Scan(
		&deviceRow.ID,
		&deviceRow.Name,
		&deviceRow.Brand,
		&deviceRow.State,
		&deviceRow.CreatedAt,
		&deviceRow.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return r.convertRowToDevice(deviceRow)
}

func (r *DevicesRepository) scanDeviceFromRows(rows pgx.Rows) (*model.Device, error) {
	var deviceRow deviceRow

	err := rows.Scan(
		&deviceRow.ID,
		&deviceRow.Name,
		&deviceRow.Brand,
		&deviceRow.State,
		&deviceRow.CreatedAt,
		&deviceRow.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return r.convertRowToDevice(deviceRow)
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
