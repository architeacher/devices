package repos

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-devices/internal/domain/model"
)

var columnMapping = map[string]string{
	"id":        "id",
	"name":      "name",
	"brand":     "brand",
	"state":     "state",
	"createdAt": "created_at",
	"updatedAt": "updated_at",
}

type CriteriaTranslator struct {
	logger *logger.Logger
}

func NewCriteriaTranslator(log *logger.Logger) *CriteriaTranslator {
	return &CriteriaTranslator{logger: log}
}

func (t *CriteriaTranslator) ApplyToSelect(builder sq.SelectBuilder, criteria model.Criteria) sq.SelectBuilder {
	if criteria.HasSpec() {
		builder = builder.Where(t.translateSpec(criteria.Spec()))
	}

	builder = t.applySorting(builder, criteria)
	builder = t.applyPagination(builder, criteria)

	return builder
}

func (t *CriteriaTranslator) ApplyConditionsOnly(builder sq.SelectBuilder, criteria model.Criteria) sq.SelectBuilder {
	if criteria.HasSpec() {
		builder = builder.Where(t.translateSpec(criteria.Spec()))
	}

	return builder
}

func (t *CriteriaTranslator) translateSpec(spec model.Specification) sq.Sqlizer {
	switch spec.Operator() {
	case model.SpecOpEq:
		return sq.Eq{t.col(spec.Field()): spec.Value()}

	case model.SpecOpIn:
		return sq.Eq{t.col(spec.Field()): spec.Value()}

	case model.SpecOpLike:
		return sq.Like{t.col(spec.Field()): spec.Value()}

	case model.SpecOpFullText:
		return sq.Expr("search_vector @@ plainto_tsquery('english', ?)", spec.Value())

	case model.SpecOpBetween:
		values := spec.Value().([]any)
		col := t.col(spec.Field())

		return sq.And{sq.GtOrEq{col: values[0]}, sq.LtOrEq{col: values[1]}}

	case model.SpecOpMust:
		conditions := make(sq.And, 0, len(spec.Children()))
		for _, child := range spec.Children() {
			conditions = append(conditions, t.translateSpec(child))
		}

		return conditions

	case model.SpecOpShould:
		conditions := make(sq.Or, 0, len(spec.Children()))
		for _, child := range spec.Children() {
			conditions = append(conditions, t.translateSpec(child))
		}

		return conditions

	case model.SpecOpMustNot:
		children := spec.Children()
		if len(children) > 0 {
			return sq.Expr("NOT (?)", t.translateSpec(children[0]))
		}
	}

	return nil
}

func (t *CriteriaTranslator) col(field string) string {
	if col, ok := columnMapping[field]; ok {
		return col
	}

	if t.logger != nil {
		t.logger.Warn().
			Str("field", field).
			Str("fallback", "created_at").
			Msg("unknown sort field requested, falling back to default")
	}

	return "created_at"
}

func (t *CriteriaTranslator) applySorting(builder sq.SelectBuilder, c model.Criteria) sq.SelectBuilder {
	if !c.HasSorting() {
		return builder.OrderBy("created_at DESC")
	}

	for _, s := range c.Sorting() {
		builder = builder.OrderBy(fmt.Sprintf("%s %s", t.col(s.Field), s.Direction))
	}

	return builder
}

func (t *CriteriaTranslator) applyPagination(builder sq.SelectBuilder, c model.Criteria) sq.SelectBuilder {
	if !c.HasPagination() {
		return builder
	}

	if c.HasCursor() {
		builder = t.applyCursorPagination(builder, c)

		return builder.Limit(uint64(c.Size()))
	}

	return builder.Limit(uint64(c.Size())).Offset(uint64(c.Offset()))
}

func (t *CriteriaTranslator) applyCursorPagination(builder sq.SelectBuilder, c model.Criteria) sq.SelectBuilder {
	cursor := c.Cursor()
	if cursor == nil {
		return builder
	}

	cursorValue, err := cursor.ParseCursorValue()
	if err != nil {
		return builder
	}

	sortField := t.normalizeSortField(cursor.Field)
	col := t.col(sortField)
	isDescending := len(cursor.Field) > 0 && cursor.Field[0] == '-'

	if cursor.Direction == model.CursorDirectionNext {
		if isDescending {
			return builder.Where(
				sq.Expr("("+col+", id) < (?, ?)", cursorValue, cursor.ID),
			)
		}

		return builder.Where(
			sq.Expr("("+col+", id) > (?, ?)", cursorValue, cursor.ID),
		)
	}

	if isDescending {
		return builder.Where(
			sq.Expr("("+col+", id) > (?, ?)", cursorValue, cursor.ID),
		)
	}

	return builder.Where(
		sq.Expr("("+col+", id) < (?, ?)", cursorValue, cursor.ID),
	)
}

func (t *CriteriaTranslator) normalizeSortField(field string) string {
	if len(field) > 0 && field[0] == '-' {
		return field[1:]
	}

	return field
}
