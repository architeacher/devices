package model

type baseSpec struct {
	self Specification
}

func (b *baseSpec) setSelf(s Specification) { b.self = s }

func (b *baseSpec) Must(other Specification) Specification {
	return &mustSpec{specs: []Specification{b.self, other}}
}
func (b *baseSpec) Should(other Specification) Specification {
	return &shouldSpec{specs: []Specification{b.self, other}}
}
func (b *baseSpec) MustNot() Specification    { return &mustNotSpec{spec: b.self} }
func (b *baseSpec) IsComposite() bool         { return false }
func (b *baseSpec) Children() []Specification { return nil }

type eqSpec struct {
	baseSpec
	field string
	value any
}

func Eq(field string, value any) Specification {
	s := &eqSpec{field: field, value: value}
	s.setSelf(s)

	return s
}

func (s *eqSpec) Operator() SpecOperator { return SpecOpEq }
func (s *eqSpec) Field() string          { return s.field }
func (s *eqSpec) Value() any             { return s.value }

type inSpec struct {
	baseSpec
	field  string
	values []any
}

func In(field string, values ...any) Specification {
	s := &inSpec{field: field, values: values}
	s.setSelf(s)

	return s
}

func (s *inSpec) Operator() SpecOperator { return SpecOpIn }
func (s *inSpec) Field() string          { return s.field }
func (s *inSpec) Value() any             { return s.values }

type likeSpec struct {
	baseSpec
	field   string
	pattern string
}

func Like(field, pattern string) Specification {
	s := &likeSpec{field: field, pattern: pattern}
	s.setSelf(s)

	return s
}

func (s *likeSpec) Operator() SpecOperator { return SpecOpLike }
func (s *likeSpec) Field() string          { return s.field }
func (s *likeSpec) Value() any             { return s.pattern }

type betweenSpec struct {
	baseSpec
	field string
	start any
	end   any
}

func Between(field string, start, end any) Specification {
	s := &betweenSpec{field: field, start: start, end: end}
	s.setSelf(s)

	return s
}

func (s *betweenSpec) Operator() SpecOperator { return SpecOpBetween }
func (s *betweenSpec) Field() string          { return s.field }
func (s *betweenSpec) Value() any             { return []any{s.start, s.end} }
