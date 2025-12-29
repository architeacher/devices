package model

type mustSpec struct {
	specs []Specification
}

func Must(specs ...Specification) Specification {
	return &mustSpec{specs: specs}
}

func (s *mustSpec) Must(other Specification) Specification {
	return &mustSpec{specs: append(s.specs, other)}
}

func (s *mustSpec) Should(other Specification) Specification {
	return &shouldSpec{specs: []Specification{s, other}}
}

func (s *mustSpec) MustNot() Specification    { return &mustNotSpec{spec: s} }
func (s *mustSpec) IsComposite() bool         { return true }
func (s *mustSpec) Children() []Specification { return s.specs }
func (s *mustSpec) Operator() SpecOperator    { return SpecOpMust }
func (s *mustSpec) Field() string             { return "" }
func (s *mustSpec) Value() any                { return nil }

type shouldSpec struct {
	specs []Specification
}

func Should(specs ...Specification) Specification {
	return &shouldSpec{specs: specs}
}

func (s *shouldSpec) Must(other Specification) Specification {
	return &mustSpec{specs: []Specification{s, other}}
}

func (s *shouldSpec) Should(other Specification) Specification {
	return &shouldSpec{specs: append(s.specs, other)}
}

func (s *shouldSpec) MustNot() Specification    { return &mustNotSpec{spec: s} }
func (s *shouldSpec) IsComposite() bool         { return true }
func (s *shouldSpec) Children() []Specification { return s.specs }
func (s *shouldSpec) Operator() SpecOperator    { return SpecOpShould }
func (s *shouldSpec) Field() string             { return "" }
func (s *shouldSpec) Value() any                { return nil }

type mustNotSpec struct {
	spec Specification
}

func MustNot(spec Specification) Specification {
	return &mustNotSpec{spec: spec}
}

func (s *mustNotSpec) Must(other Specification) Specification {
	return &mustSpec{specs: []Specification{s, other}}
}

func (s *mustNotSpec) Should(other Specification) Specification {
	return &shouldSpec{specs: []Specification{s, other}}
}

func (s *mustNotSpec) MustNot() Specification    { return s.spec }
func (s *mustNotSpec) IsComposite() bool         { return true }
func (s *mustNotSpec) Children() []Specification { return []Specification{s.spec} }
func (s *mustNotSpec) Operator() SpecOperator    { return SpecOpMustNot }
func (s *mustNotSpec) Field() string             { return "" }
func (s *mustNotSpec) Value() any                { return nil }
