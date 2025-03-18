package queryparser

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
)

const (
	selectQuery int64 = iota
	updateQuery
	deleteQuery
	insertQuery
)

// SqlBuilder wraps Squirrel query builders and provides methods to apply
// filters, options, and model to the query.
type SqlBuilder struct {
	queryType     int64
	selectBuilder squirrel.SelectBuilder
	updateBuilder squirrel.UpdateBuilder
	deleteBuilder squirrel.DeleteBuilder
	insertBuilder squirrel.InsertBuilder
	ctx           context.Context
}

// ToSql returns the SQL query string and arguments from the underlying Squirrel
// builder
func (qb *SqlBuilder) ToSql() (string, []interface{}, error) {
	switch qb.queryType {
	case selectQuery:
		return qb.selectBuilder.ToSql()
	case updateQuery:
		return qb.updateBuilder.ToSql()
	case deleteQuery:
		return qb.deleteBuilder.ToSql()
	case insertQuery:
		return qb.insertBuilder.ToSql()
	default:
		return "", nil, fmt.Errorf("invalid query type")
	}
}

// Apply applies the filters and options to the QueryBuilder
func (qb *SqlBuilder) Apply(filters []Filter, options *QueryOptions, model interface{}) (*SqlBuilder, error) {
	// Get JSON tags from the model
	tags, err := getJSONTags(model)
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON tags: %w", err)
	}

	// Validate fields against JSON tags
	if err := validateFields(filters, options, tags); err != nil {
		return nil, err
	}

	if qb.selectBuilder != (squirrel.SelectBuilder{}) {
		qb, err := qb.applySelectFilters(filters)
		if err != nil {
			return nil, err
		}
		return qb.applyOptions(options)
	}
	// Add support for other query types as needed
	return qb, nil
}

func (qb *SqlBuilder) SelectBuilder() squirrel.SelectBuilder {
	return qb.selectBuilder
}

func (qb *SqlBuilder) UpdateBuilder() squirrel.UpdateBuilder {
	return qb.updateBuilder
}

func (qb *SqlBuilder) DeleteBuilder() squirrel.DeleteBuilder {
	return qb.deleteBuilder
}

// applySelectFilters applies filters to a SELECT query
func (qb *SqlBuilder) applySelectFilters(filters []Filter) (*SqlBuilder, error) {
	conditions := make([]squirrel.Sqlizer, 0, len(filters))

	for _, filter := range filters {
		condition, err := qb.buildCondition(filter)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}

	if len(conditions) > 0 {
		qb.selectBuilder = qb.selectBuilder.Where(squirrel.And(conditions))
	}

	return qb, nil
}

// buildCondition converts a Filter into a Squirrel condition
func (qb *SqlBuilder) buildCondition(filter Filter) (squirrel.Sqlizer, error) {
	switch filter.Operator {
	case OpEq:
		return squirrel.Eq{filter.Field: filter.Value}, nil
	case OpNe:
		return squirrel.NotEq{filter.Field: filter.Value}, nil
	case OpLt:
		return squirrel.Lt{filter.Field: filter.Value}, nil
	case OpLte:
		return squirrel.LtOrEq{filter.Field: filter.Value}, nil
	case OpGt:
		return squirrel.Gt{filter.Field: filter.Value}, nil
	case OpGte:
		return squirrel.GtOrEq{filter.Field: filter.Value}, nil
	case OpIn:
		return squirrel.Eq{filter.Field: filter.Value}, nil
	case OpNin:
		return squirrel.NotEq{filter.Field: filter.Value}, nil
	default:
		return nil, fmt.Errorf("unsupported operator: %s", filter.Operator)
	}
}

// applyOptions applies sorting and pagination options to the query
func (qb *SqlBuilder) applyOptions(options *QueryOptions) (*SqlBuilder, error) {
	if options == nil {
		return qb, nil
	}

	// Apply sorting
	if len(options.Sort) > 0 {
		for field, direction := range options.Sort {
			if direction == SortDesc {
				qb.selectBuilder = qb.selectBuilder.OrderBy(field + " DESC")
			} else {
				qb.selectBuilder = qb.selectBuilder.OrderBy(field + " ASC")
			}
		}
	}

	// Apply pagination
	if options.Limit != nil {
		qb.selectBuilder = qb.selectBuilder.Limit(uint64(*options.Limit))
	}
	if options.Offset != nil {
		qb.selectBuilder = qb.selectBuilder.Offset(uint64(*options.Offset))
	}

	return qb, nil
}

// NewSqlBuilder creates a new SqlBuilder instance
func NewSqlBuilder(ctx context.Context) *SqlBuilder {
	return &SqlBuilder{
		ctx: ctx,
	}
}

// WithSelect sets up the QueryBuilder for SELECT operations
func (qb *SqlBuilder) WithSelect(table string) *SqlBuilder {
	qb.selectBuilder = squirrel.Select("*").From(table)
	qb.queryType = selectQuery
	return qb
}

// WithUpdate sets up the QueryBuilder for UPDATE operations
func (qb *SqlBuilder) WithUpdate(table string) *SqlBuilder {
	qb.updateBuilder = squirrel.Update(table)
	qb.queryType = updateQuery
	return qb
}

// WithDelete sets up the QueryBuilder for DELETE operations
func (qb *SqlBuilder) WithDelete(table string) *SqlBuilder {
	qb.deleteBuilder = squirrel.Delete(table)
	qb.queryType = deleteQuery
	return qb
}

// WithInsert sets up the QueryBuilder for INSERT operations
func (qb *SqlBuilder) WithInsert(table string) *SqlBuilder {
	qb.insertBuilder = squirrel.Insert(table)
	qb.queryType = insertQuery
	return qb
}
