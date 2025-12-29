package model

type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"

	defaultPage uint = 1
	defaultSize uint = 20
)

type (
	SortField struct {
		Field     string
		Direction SortDirection
	}

	Criteria struct {
		spec    Specification
		sorting []SortField
		page    uint
		size    uint
		fields  []string
	}
)

func (c Criteria) Spec() Specification  { return c.spec }
func (c Criteria) Sorting() []SortField { return c.sorting }
func (c Criteria) Page() uint           { return c.page }
func (c Criteria) Size() uint           { return c.size }
func (c Criteria) Fields() []string     { return c.fields }
func (c Criteria) Offset() uint         { return (c.page - 1) * c.size }
func (c Criteria) HasSpec() bool        { return c.spec != nil }
func (c Criteria) HasSorting() bool     { return len(c.sorting) > 0 }
func (c Criteria) HasPagination() bool  { return c.page > 0 && c.size > 0 }

func FromDeviceFilter(filter DeviceFilter) Criteria {
	builder := NewCriteria()

	if len(filter.Brands) > 0 {
		builder.WhereIn("brand", toAnySlice(filter.Brands)...)
	}

	if len(filter.States) > 0 {
		stateStrings := make([]any, 0, len(filter.States))
		for _, s := range filter.States {
			stateStrings = append(stateStrings, s.String())
		}

		builder.WhereIn("state", stateStrings...)
	}

	if len(filter.Sort) > 0 {
		for _, sort := range filter.Sort {
			builder.OrderBy(sort)
		}
	} else {
		builder.OrderBy("-createdAt")
	}

	builder.Paginate(filter.Page, filter.Size)

	return builder.Build()
}

func toAnySlice(strings []string) []any {
	result := make([]any, len(strings))
	for index, s := range strings {
		result[index] = s
	}

	return result
}
