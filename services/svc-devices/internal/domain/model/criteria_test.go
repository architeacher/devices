package model_test

import (
	"testing"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestCriteriaBuilder_Where(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		Where("brand", "Apple").
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpEq, criteria.Spec().Operator())
	require.Equal(t, "brand", criteria.Spec().Field())
	require.Equal(t, "Apple", criteria.Spec().Value())
}

func TestCriteriaBuilder_WhereIn(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereIn("state", "available", "in-use").
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpIn, criteria.Spec().Operator())
	require.Equal(t, "state", criteria.Spec().Field())
	require.Equal(t, []any{"available", "in-use"}, criteria.Spec().Value())
}

func TestCriteriaBuilder_WhereLike(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereLike("name", "%Pro%").
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpLike, criteria.Spec().Operator())
	require.Equal(t, "name", criteria.Spec().Field())
	require.Equal(t, "%Pro%", criteria.Spec().Value())
}

func TestCriteriaBuilder_WhereBetween(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereBetween("price", 100, 500).
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpBetween, criteria.Spec().Operator())
	require.Equal(t, "price", criteria.Spec().Field())
	require.Equal(t, []any{100, 500}, criteria.Spec().Value())
}

func TestCriteriaBuilder_MultipleConditions(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		Where("brand", "Apple").
		WhereIn("state", "available", "in-use").
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpMust, criteria.Spec().Operator())
	require.True(t, criteria.Spec().IsComposite())
	require.Len(t, criteria.Spec().Children(), 2)
}

func TestCriteriaBuilder_WhereSpec(t *testing.T) {
	t.Parallel()

	spec := model.Should(
		model.Eq("brand", "Apple"),
		model.Eq("brand", "Samsung"),
	)

	criteria := model.NewCriteria().
		WhereSpec(spec).
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpShould, criteria.Spec().Operator())
}

func TestCriteriaBuilder_WhereMustNot(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereMustNot(model.Like("name", "%Test%")).
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpMustNot, criteria.Spec().Operator())
}

func TestCriteriaBuilder_WhereShould(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereShould(
			model.Eq("brand", "Apple"),
			model.Eq("brand", "Samsung"),
		).
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpShould, criteria.Spec().Operator())
	require.Len(t, criteria.Spec().Children(), 2)
}

func TestCriteriaBuilder_WhereMust(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereMust(
			model.Eq("brand", "Apple"),
			model.In("state", "available"),
		).
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpMust, criteria.Spec().Operator())
	require.Len(t, criteria.Spec().Children(), 2)
}

func TestCriteriaBuilder_OrderBy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		sortField         string
		expectedField     string
		expectedDirection model.SortDirection
	}{
		{
			name:              "ascending",
			sortField:         "name",
			expectedField:     "name",
			expectedDirection: model.SortAsc,
		},
		{
			name:              "descending",
			sortField:         "-createdAt",
			expectedField:     "createdAt",
			expectedDirection: model.SortDesc,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			criteria := model.NewCriteria().
				OrderBy(tc.sortField).
				Build()

			require.True(t, criteria.HasSorting())
			require.Len(t, criteria.Sorting(), 1)
			require.Equal(t, tc.expectedField, criteria.Sorting()[0].Field)
			require.Equal(t, tc.expectedDirection, criteria.Sorting()[0].Direction)
		})
	}
}

func TestCriteriaBuilder_MultipleSorting(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		OrderBy("-createdAt").
		OrderBy("name").
		Build()

	require.True(t, criteria.HasSorting())
	require.Len(t, criteria.Sorting(), 2)
	require.Equal(t, "createdAt", criteria.Sorting()[0].Field)
	require.Equal(t, model.SortDesc, criteria.Sorting()[0].Direction)
	require.Equal(t, "name", criteria.Sorting()[1].Field)
	require.Equal(t, model.SortAsc, criteria.Sorting()[1].Direction)
}

func TestCriteriaBuilder_Paginate(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		Paginate(2, 25).
		Build()

	require.True(t, criteria.HasPagination())
	require.Equal(t, uint(2), criteria.Page())
	require.Equal(t, uint(25), criteria.Size())
	require.Equal(t, uint(25), criteria.Offset())
}

func TestCriteriaBuilder_PaginateDefaults(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().Build()

	require.True(t, criteria.HasPagination())
	require.Equal(t, uint(1), criteria.Page())
	require.Equal(t, uint(20), criteria.Size())
	require.Equal(t, uint(0), criteria.Offset())
}

func TestCriteriaBuilder_PaginateIgnoresZeroValues(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		Paginate(0, 0).
		Build()

	require.Equal(t, uint(1), criteria.Page())
	require.Equal(t, uint(20), criteria.Size())
}

func TestCriteriaBuilder_Select(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		Select("id", "name", "brand").
		Build()

	require.Equal(t, []string{"id", "name", "brand"}, criteria.Fields())
}

func TestCriteriaBuilder_ComplexQuery(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().
		WhereSpec(model.Should(
			model.Eq("brand", "Apple"),
			model.Must(
				model.Eq("brand", "Samsung"),
				model.In("state", "available"),
			),
		)).
		WhereMustNot(model.Like("name", "%Test%")).
		OrderBy("-createdAt").
		Paginate(1, 20).
		Build()

	require.True(t, criteria.HasSpec())
	require.Equal(t, model.SpecOpMust, criteria.Spec().Operator())
	require.Len(t, criteria.Spec().Children(), 2)
	require.True(t, criteria.HasSorting())
	require.True(t, criteria.HasPagination())
}

func TestCriteria_EmptySpec(t *testing.T) {
	t.Parallel()

	criteria := model.NewCriteria().Build()

	require.False(t, criteria.HasSpec())
	require.Nil(t, criteria.Spec())
}

func TestFromDeviceFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		filter          model.DeviceFilter
		expectedHasSpec bool
		expectedPage    uint
		expectedSize    uint
	}{
		{
			name: "with brands",
			filter: model.DeviceFilter{
				Brands: []string{"Apple", "Samsung"},
				Page:   1,
				Size:   20,
			},
			expectedHasSpec: true,
			expectedPage:    1,
			expectedSize:    20,
		},
		{
			name: "with states",
			filter: model.DeviceFilter{
				States: []model.State{model.StateAvailable, model.StateInUse},
				Page:   2,
				Size:   10,
			},
			expectedHasSpec: true,
			expectedPage:    2,
			expectedSize:    10,
		},
		{
			name: "with brands and states",
			filter: model.DeviceFilter{
				Brands: []string{"Apple"},
				States: []model.State{model.StateAvailable},
				Page:   1,
				Size:   20,
			},
			expectedHasSpec: true,
			expectedPage:    1,
			expectedSize:    20,
		},
		{
			name: "empty filter",
			filter: model.DeviceFilter{
				Page: 1,
				Size: 20,
			},
			expectedHasSpec: false,
			expectedPage:    1,
			expectedSize:    20,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			criteria := model.FromDeviceFilter(tc.filter)

			require.Equal(t, tc.expectedHasSpec, criteria.HasSpec())
			require.Equal(t, tc.expectedPage, criteria.Page())
			require.Equal(t, tc.expectedSize, criteria.Size())
			require.True(t, criteria.HasSorting())
		})
	}
}

func TestFromDeviceFilter_WithSort(t *testing.T) {
	t.Parallel()

	filter := model.DeviceFilter{
		Sort: []string{"-name"},
		Page: 1,
		Size: 20,
	}

	criteria := model.FromDeviceFilter(filter)

	require.True(t, criteria.HasSorting())
	require.Len(t, criteria.Sorting(), 1)
	require.Equal(t, "name", criteria.Sorting()[0].Field)
	require.Equal(t, model.SortDesc, criteria.Sorting()[0].Direction)
}

func TestFromDeviceFilter_WithMultiFieldSort(t *testing.T) {
	t.Parallel()

	filter := model.DeviceFilter{
		Sort: []string{"-createdAt", "name", "-brand"},
		Page: 1,
		Size: 20,
	}

	criteria := model.FromDeviceFilter(filter)

	require.True(t, criteria.HasSorting())
	require.Len(t, criteria.Sorting(), 3)

	require.Equal(t, "createdAt", criteria.Sorting()[0].Field)
	require.Equal(t, model.SortDesc, criteria.Sorting()[0].Direction)

	require.Equal(t, "name", criteria.Sorting()[1].Field)
	require.Equal(t, model.SortAsc, criteria.Sorting()[1].Direction)

	require.Equal(t, "brand", criteria.Sorting()[2].Field)
	require.Equal(t, model.SortDesc, criteria.Sorting()[2].Direction)
}

func TestFromDeviceFilter_DefaultSort(t *testing.T) {
	t.Parallel()

	filter := model.DeviceFilter{
		Page: 1,
		Size: 20,
	}

	criteria := model.FromDeviceFilter(filter)

	require.True(t, criteria.HasSorting())
	require.Len(t, criteria.Sorting(), 1)
	require.Equal(t, "createdAt", criteria.Sorting()[0].Field)
	require.Equal(t, model.SortDesc, criteria.Sorting()[0].Direction)
}
