package repos_test

import (
	"testing"

	sq "github.com/Masterminds/squirrel"
	"github.com/architeacher/devices/services/svc-devices/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/stretchr/testify/require"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func TestCriteriaTranslator_EqSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		Where("brand", "Apple").
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "brand = $1")
	require.Equal(t, []any{"Apple"}, args)
}

func TestCriteriaTranslator_InSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		WhereIn("state", "available", "in-use").
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "state IN ($1,$2)")
	require.Equal(t, []any{"available", "in-use"}, args)
}

func TestCriteriaTranslator_LikeSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		WhereLike("name", "%Pro%").
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "name LIKE $1")
	require.Equal(t, []any{"%Pro%"}, args)
}

func TestCriteriaTranslator_BetweenSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		WhereBetween("createdAt", "2024-01-01", "2024-12-31").
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "created_at >= $1")
	require.Contains(t, sql, "created_at <= $2")
	require.Equal(t, []any{"2024-01-01", "2024-12-31"}, args)
}

func TestCriteriaTranslator_MustSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		Where("brand", "Apple").
		Where("state", "available").
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "brand = $1")
	require.Contains(t, sql, "state = $2")
	require.Contains(t, sql, "AND")
	require.Equal(t, []any{"Apple", "available"}, args)
}

func TestCriteriaTranslator_ShouldSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		WhereShould(
			model.Eq("brand", "Apple"),
			model.Eq("brand", "Samsung"),
		).
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "brand = $1")
	require.Contains(t, sql, "brand = $2")
	require.Contains(t, sql, "OR")
	require.Equal(t, []any{"Apple", "Samsung"}, args)
}

func TestCriteriaTranslator_MustNotSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		WhereMustNot(model.Like("name", "%Test%")).
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "NOT")
	require.Contains(t, sql, "name LIKE $1")
	require.Equal(t, []any{"%Test%"}, args)
}

func TestCriteriaTranslator_NestedSpec(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		WhereSpec(model.Should(
			model.Eq("brand", "Apple"),
			model.Must(
				model.Eq("brand", "Samsung"),
				model.In("state", "available"),
			),
		)).
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "OR")
	require.Contains(t, sql, "AND")
	require.Len(t, args, 3)
}

func TestCriteriaTranslator_ColumnMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		field         string
		expectedField string
	}{
		{
			name:          "maps createdAt to created_at",
			field:         "createdAt",
			expectedField: "created_at",
		},
		{
			name:          "maps unknown field to created_at (fallback)",
			field:         "unknownField",
			expectedField: "created_at",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			translator := repos.NewCriteriaTranslator(nil)
			criteria := model.NewCriteria().
				Where(tc.field, "2024-01-01").
				Build()

			builder := psql.Select("*").From("devices")
			builder = translator.ApplyConditionsOnly(builder, criteria)

			sql, _, err := builder.ToSql()

			require.NoError(t, err)
			require.Contains(t, sql, tc.expectedField+" = $1")
		})
	}
}

func TestCriteriaTranslator_Sorting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		sortField     string
		expectedOrder string
	}{
		{
			name:          "ascending",
			sortField:     "name",
			expectedOrder: "name ASC",
		},
		{
			name:          "descending",
			sortField:     "-createdAt",
			expectedOrder: "created_at DESC",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			translator := repos.NewCriteriaTranslator(nil)
			criteria := model.NewCriteria().
				OrderBy(tc.sortField).
				Build()

			builder := psql.Select("*").From("devices")
			builder = translator.ApplyToSelect(builder, criteria)

			sql, _, err := builder.ToSql()

			require.NoError(t, err)
			require.Contains(t, sql, tc.expectedOrder)
		})
	}
}

func TestCriteriaTranslator_DefaultSorting(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyToSelect(builder, criteria)

	sql, _, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "ORDER BY created_at DESC")
}

func TestCriteriaTranslator_Pagination(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		Paginate(2, 25).
		Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyToSelect(builder, criteria)

	sql, _, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "LIMIT 25")
	require.Contains(t, sql, "OFFSET 25")
}

func TestCriteriaTranslator_EmptyCriteria(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().Build()

	builder := psql.Select("*").From("devices")
	builder = translator.ApplyConditionsOnly(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM devices", sql)
	require.Empty(t, args)
}

func TestCriteriaTranslator_FullQuery(t *testing.T) {
	t.Parallel()

	translator := repos.NewCriteriaTranslator(nil)
	criteria := model.NewCriteria().
		Where("brand", "Apple").
		WhereIn("state", "available", "in-use").
		OrderBy("-createdAt").
		Paginate(1, 20).
		Build()

	builder := psql.Select("id", "name", "brand").From("devices")
	builder = translator.ApplyToSelect(builder, criteria)

	sql, args, err := builder.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "SELECT id, name, brand FROM devices")
	require.Contains(t, sql, "brand = $1")
	require.Contains(t, sql, "state IN ($2,$3)")
	require.Contains(t, sql, "ORDER BY created_at DESC")
	require.Contains(t, sql, "LIMIT 20")
	require.Contains(t, sql, "OFFSET 0")
	require.Equal(t, []any{"Apple", "available", "in-use"}, args)
}
