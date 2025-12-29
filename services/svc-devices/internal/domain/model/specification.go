package model

type SpecOperator string

const (
	SpecOpEq      SpecOperator = "eq"
	SpecOpNotEq   SpecOperator = "neq"
	SpecOpIn      SpecOperator = "in"
	SpecOpNotIn   SpecOperator = "not_in"
	SpecOpLike    SpecOperator = "like"
	SpecOpILike   SpecOperator = "ilike"
	SpecOpGt      SpecOperator = "gt"
	SpecOpGte     SpecOperator = "gte"
	SpecOpLt      SpecOperator = "lt"
	SpecOpLte     SpecOperator = "lte"
	SpecOpBetween SpecOperator = "between"
	SpecOpIsNull  SpecOperator = "is_null"
	SpecOpNotNull SpecOperator = "not_null"
	SpecOpMust    SpecOperator = "must"
	SpecOpShould  SpecOperator = "should"
	SpecOpMustNot SpecOperator = "must_not"
)

type Specification interface {
	Must(other Specification) Specification
	Should(other Specification) Specification
	MustNot() Specification
	IsComposite() bool
	Children() []Specification
	Operator() SpecOperator
	Field() string
	Value() any
}
