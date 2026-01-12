package model

type CriteriaBuilder struct {
	specs   []Specification
	sorting []SortField
	page    uint
	size    uint
	fields  []string
	cursor  *Cursor
}

func NewCriteria() *CriteriaBuilder {
	return &CriteriaBuilder{
		specs: make([]Specification, 0),
		page:  defaultPage,
		size:  defaultSize,
	}
}

func (b *CriteriaBuilder) Where(field string, value any) *CriteriaBuilder {
	b.specs = append(b.specs, Eq(field, value))

	return b
}

func (b *CriteriaBuilder) WhereIn(field string, values ...any) *CriteriaBuilder {
	b.specs = append(b.specs, In(field, values...))

	return b
}

func (b *CriteriaBuilder) WhereLike(field, pattern string) *CriteriaBuilder {
	b.specs = append(b.specs, Like(field, pattern))

	return b
}

func (b *CriteriaBuilder) WhereBetween(field string, start, end any) *CriteriaBuilder {
	b.specs = append(b.specs, Between(field, start, end))

	return b
}

func (b *CriteriaBuilder) WhereFullText(query string) *CriteriaBuilder {
	b.specs = append(b.specs, FullText(query))

	return b
}

func (b *CriteriaBuilder) WhereSpec(spec Specification) *CriteriaBuilder {
	b.specs = append(b.specs, spec)

	return b
}

func (b *CriteriaBuilder) WhereMustNot(spec Specification) *CriteriaBuilder {
	b.specs = append(b.specs, MustNot(spec))

	return b
}

func (b *CriteriaBuilder) WhereShould(specs ...Specification) *CriteriaBuilder {
	b.specs = append(b.specs, Should(specs...))

	return b
}

func (b *CriteriaBuilder) WhereMust(specs ...Specification) *CriteriaBuilder {
	b.specs = append(b.specs, Must(specs...))

	return b
}

func (b *CriteriaBuilder) OrderBy(field string) *CriteriaBuilder {
	direction := SortAsc
	actualField := field

	if len(field) > 0 && field[0] == '-' {
		direction = SortDesc
		actualField = field[1:]
	}

	b.sorting = append(b.sorting, SortField{Field: actualField, Direction: direction})

	return b
}

func (b *CriteriaBuilder) Paginate(page, size uint) *CriteriaBuilder {
	if page > 0 {
		b.page = page
	}

	if size > 0 {
		b.size = size
	}

	return b
}

func (b *CriteriaBuilder) Select(fields ...string) *CriteriaBuilder {
	b.fields = append(b.fields, fields...)

	return b
}

func (b *CriteriaBuilder) WithCursor(cursor *Cursor) *CriteriaBuilder {
	b.cursor = cursor

	return b
}

func (b *CriteriaBuilder) Build() Criteria {
	var rootSpec Specification

	if len(b.specs) == 1 {
		rootSpec = b.specs[0]
	} else if len(b.specs) > 1 {
		rootSpec = Must(b.specs...)
	}

	return Criteria{
		spec:    rootSpec,
		sorting: b.sorting,
		page:    b.page,
		size:    b.size,
		fields:  b.fields,
		cursor:  b.cursor,
	}
}
