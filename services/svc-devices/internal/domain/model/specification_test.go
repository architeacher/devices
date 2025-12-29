package model_test

import (
	"testing"

	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestEqSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		field         string
		value         any
		expectedOp    model.SpecOperator
		expectedField string
		expectedValue any
	}{
		{
			name:          "string value",
			field:         "brand",
			value:         "Apple",
			expectedOp:    model.SpecOpEq,
			expectedField: "brand",
			expectedValue: "Apple",
		},
		{
			name:          "integer value",
			field:         "count",
			value:         42,
			expectedOp:    model.SpecOpEq,
			expectedField: "count",
			expectedValue: 42,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := model.Eq(tc.field, tc.value)

			require.Equal(t, tc.expectedOp, spec.Operator())
			require.Equal(t, tc.expectedField, spec.Field())
			require.Equal(t, tc.expectedValue, spec.Value())
			require.False(t, spec.IsComposite())
			require.Nil(t, spec.Children())
		})
	}
}

func TestInSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		field         string
		values        []any
		expectedOp    model.SpecOperator
		expectedField string
	}{
		{
			name:          "single value",
			field:         "state",
			values:        []any{"available"},
			expectedOp:    model.SpecOpIn,
			expectedField: "state",
		},
		{
			name:          "multiple values",
			field:         "brand",
			values:        []any{"Apple", "Samsung", "Google"},
			expectedOp:    model.SpecOpIn,
			expectedField: "brand",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := model.In(tc.field, tc.values...)

			require.Equal(t, tc.expectedOp, spec.Operator())
			require.Equal(t, tc.expectedField, spec.Field())
			require.Equal(t, tc.values, spec.Value())
			require.False(t, spec.IsComposite())
		})
	}
}

func TestLikeSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		field           string
		pattern         string
		expectedOp      model.SpecOperator
		expectedField   string
		expectedPattern string
	}{
		{
			name:            "prefix pattern",
			field:           "name",
			pattern:         "iPhone%",
			expectedOp:      model.SpecOpLike,
			expectedField:   "name",
			expectedPattern: "iPhone%",
		},
		{
			name:            "contains pattern",
			field:           "name",
			pattern:         "%Pro%",
			expectedOp:      model.SpecOpLike,
			expectedField:   "name",
			expectedPattern: "%Pro%",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := model.Like(tc.field, tc.pattern)

			require.Equal(t, tc.expectedOp, spec.Operator())
			require.Equal(t, tc.expectedField, spec.Field())
			require.Equal(t, tc.expectedPattern, spec.Value())
			require.False(t, spec.IsComposite())
		})
	}
}

func TestBetweenSpec(t *testing.T) {
	t.Parallel()

	spec := model.Between("price", 100, 500)

	require.Equal(t, model.SpecOpBetween, spec.Operator())
	require.Equal(t, "price", spec.Field())
	require.Equal(t, []any{100, 500}, spec.Value())
	require.False(t, spec.IsComposite())
}

func TestMustSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		specs            []model.Specification
		expectedOp       model.SpecOperator
		expectedChildren int
	}{
		{
			name: "two specs",
			specs: []model.Specification{
				model.Eq("brand", "Apple"),
				model.Eq("state", "available"),
			},
			expectedOp:       model.SpecOpMust,
			expectedChildren: 2,
		},
		{
			name: "three specs",
			specs: []model.Specification{
				model.Eq("brand", "Apple"),
				model.In("state", "available", "in-use"),
				model.Like("name", "%Pro%"),
			},
			expectedOp:       model.SpecOpMust,
			expectedChildren: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := model.Must(tc.specs...)

			require.Equal(t, tc.expectedOp, spec.Operator())
			require.True(t, spec.IsComposite())
			require.Len(t, spec.Children(), tc.expectedChildren)
			require.Empty(t, spec.Field())
			require.Nil(t, spec.Value())
		})
	}
}

func TestShouldSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		specs            []model.Specification
		expectedOp       model.SpecOperator
		expectedChildren int
	}{
		{
			name: "two specs",
			specs: []model.Specification{
				model.Eq("brand", "Apple"),
				model.Eq("brand", "Samsung"),
			},
			expectedOp:       model.SpecOpShould,
			expectedChildren: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := model.Should(tc.specs...)

			require.Equal(t, tc.expectedOp, spec.Operator())
			require.True(t, spec.IsComposite())
			require.Len(t, spec.Children(), tc.expectedChildren)
			require.Empty(t, spec.Field())
			require.Nil(t, spec.Value())
		})
	}
}

func TestMustNotSpec(t *testing.T) {
	t.Parallel()

	innerSpec := model.Like("name", "%Test%")
	spec := model.MustNot(innerSpec)

	require.Equal(t, model.SpecOpMustNot, spec.Operator())
	require.True(t, spec.IsComposite())
	require.Len(t, spec.Children(), 1)
	require.Equal(t, innerSpec, spec.Children()[0])
	require.Empty(t, spec.Field())
	require.Nil(t, spec.Value())
}

func TestNestedComposition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		buildSpec    func() model.Specification
		expectedOp   model.SpecOperator
		verifyNested func(t *testing.T, spec model.Specification)
	}{
		{
			name: "should with nested must",
			buildSpec: func() model.Specification {
				return model.Should(
					model.Eq("brand", "Apple"),
					model.Must(
						model.Eq("brand", "Samsung"),
						model.In("state", "available"),
					),
				)
			},
			expectedOp: model.SpecOpShould,
			verifyNested: func(t *testing.T, spec model.Specification) {
				children := spec.Children()
				require.Len(t, children, 2)
				require.Equal(t, model.SpecOpEq, children[0].Operator())
				require.Equal(t, model.SpecOpMust, children[1].Operator())
				require.Len(t, children[1].Children(), 2)
			},
		},
		{
			name: "must with must_not",
			buildSpec: func() model.Specification {
				return model.Must(
					model.Eq("brand", "Apple"),
					model.MustNot(model.Like("name", "%Test%")),
				)
			},
			expectedOp: model.SpecOpMust,
			verifyNested: func(t *testing.T, spec model.Specification) {
				children := spec.Children()
				require.Len(t, children, 2)
				require.Equal(t, model.SpecOpEq, children[0].Operator())
				require.Equal(t, model.SpecOpMustNot, children[1].Operator())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := tc.buildSpec()

			require.Equal(t, tc.expectedOp, spec.Operator())
			tc.verifyNested(t, spec)
		})
	}
}

func TestChainedMethods(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		buildSpec  func() model.Specification
		expectedOp model.SpecOperator
	}{
		{
			name: "eq must eq",
			buildSpec: func() model.Specification {
				return model.Eq("brand", "Apple").Must(model.Eq("state", "available"))
			},
			expectedOp: model.SpecOpMust,
		},
		{
			name: "eq should eq",
			buildSpec: func() model.Specification {
				return model.Eq("brand", "Apple").Should(model.Eq("brand", "Samsung"))
			},
			expectedOp: model.SpecOpShould,
		},
		{
			name: "eq must_not",
			buildSpec: func() model.Specification {
				return model.Eq("brand", "Apple").MustNot()
			},
			expectedOp: model.SpecOpMustNot,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := tc.buildSpec()

			require.Equal(t, tc.expectedOp, spec.Operator())
			require.True(t, spec.IsComposite())
		})
	}
}

func TestDoubleMustNot(t *testing.T) {
	t.Parallel()

	innerSpec := model.Eq("brand", "Apple")
	doubleNegated := model.MustNot(innerSpec).MustNot()

	require.Equal(t, innerSpec, doubleNegated)
}
