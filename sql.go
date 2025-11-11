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
	queryType         int64
	selectBuilder     squirrel.SelectBuilder
	updateBuilder     squirrel.UpdateBuilder
	deleteBuilder     squirrel.DeleteBuilder
	insertBuilder     squirrel.InsertBuilder
	ctx               context.Context
	placeholderFormat squirrel.PlaceholderFormat
}

// ToSql returns the SQL query string and arguments from the underlying Squirrel
// builder
func (qb *SqlBuilder) ToSql() (string, []any, error) {
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
func (qb *SqlBuilder) Apply(filters []Filter, options *QueryOptions, model any) (*SqlBuilder, error) {
	// Get JSON tags and DB tags from the model
	jsonTags, err := getJSONTags(model)
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON tags: %w", err)
	}

	dbTags, err := getDBTags(model)
	if err != nil {
		return nil, fmt.Errorf("failed to get DB tags: %w", err)
	}

	// Create mapping from JSON field names to DB column names
	jsonToDB := make(map[string]string)
	for fieldName, jsonTag := range jsonTags {
		if dbTag, exists := dbTags[fieldName]; exists {
			jsonToDB[jsonTag] = dbTag
		}
	}

	// Validate fields against JSON tags
	if err := validateFields(filters, options, jsonTags); err != nil {
		return nil, err
	}

	if qb.selectBuilder != (squirrel.SelectBuilder{}) {
		qb, err := qb.applySelectFilters(filters, jsonToDB)
		if err != nil {
			return nil, err
		}
		return qb.applyOptions(options, jsonToDB)
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

// SetPlaceholderFormat sets the placeholder format for the SqlBuilder
//
// Example:
//
//	qb := NewSqlBuilder(ctx)
//	qb.SetPlaceholderFormat(squirrel.Question)
//	qb.WithSelect("users")
//	// Now generates SQL like: SELECT * FROM users WHERE name = ?
func (qb *SqlBuilder) SetPlaceholderFormat(format squirrel.PlaceholderFormat) {
	qb.placeholderFormat = format
}

// GetPlaceholderFormat returns the current placeholder format
func (qb *SqlBuilder) GetPlaceholderFormat() squirrel.PlaceholderFormat {
	return qb.placeholderFormat
}

// applySelectFilters applies filters to a SELECT query
func (qb *SqlBuilder) applySelectFilters(filters []Filter, jsonToDB map[string]string) (*SqlBuilder, error) {
	conditions := make([]squirrel.Sqlizer, 0, len(filters))

	for _, filter := range filters {
		condition, err := qb.buildCondition(filter, jsonToDB)
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
func (qb *SqlBuilder) buildCondition(filter Filter, jsonToDB map[string]string) (squirrel.Sqlizer, error) {
	// Map JSON field name to DB column name
	dbField := filter.Field
	if mappedField, exists := jsonToDB[filter.Field]; exists {
		dbField = mappedField
	}

	switch filter.Operator {
	case OpEq:
		return squirrel.Eq{dbField: filter.Value}, nil
	case OpNe:
		return squirrel.NotEq{dbField: filter.Value}, nil
	case OpLt:
		return squirrel.Lt{dbField: filter.Value}, nil
	case OpLte:
		return squirrel.LtOrEq{dbField: filter.Value}, nil
	case OpGt:
		return squirrel.Gt{dbField: filter.Value}, nil
	case OpGte:
		return squirrel.GtOrEq{dbField: filter.Value}, nil
	case OpIn:
		return squirrel.Eq{dbField: filter.Value}, nil
	case OpNin:
		return squirrel.NotEq{dbField: filter.Value}, nil
	case OpLike:
		// Use ILIKE for case-insensitive search in PostgreSQL
		return squirrel.Expr(dbField+" ILIKE ?", "%"+filter.Value.(string)+"%"), nil
	default:
		return nil, fmt.Errorf("unsupported operator: %s", filter.Operator)
	}
}

// applyOptions applies sorting and pagination options to the query
func (qb *SqlBuilder) applyOptions(options *QueryOptions, jsonToDB map[string]string) (*SqlBuilder, error) {
	if options == nil {
		return qb, nil
	}

	// Apply sorting
	if len(options.Sort) > 0 {
		for field, direction := range options.Sort {
			// Map JSON field name to DB column name
			dbField := field
			if mappedField, exists := jsonToDB[field]; exists {
				dbField = mappedField
			}

			if direction == SortDesc {
				qb.selectBuilder = qb.selectBuilder.OrderBy(dbField + " DESC")
			} else {
				qb.selectBuilder = qb.selectBuilder.OrderBy(dbField + " ASC")
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

// NewSqlBuilder creates a new SqlBuilder instance with default Dollar placeholder format
//
// Example:
//
//	qb := NewSqlBuilder(ctx)
//	qb.WithSelect("users")
//	// Generates SQL like: SELECT * FROM users WHERE name = $1
func NewSqlBuilder(ctx context.Context) *SqlBuilder {
	return &SqlBuilder{
		ctx:               ctx,
		placeholderFormat: squirrel.Dollar,
	}
}

// NewSqlBuilderWithPlaceholderFormat creates a new SqlBuilder instance with specified placeholder format
//
// Example:
//
//	qb := NewSqlBuilderWithPlaceholderFormat(ctx, squirrel.Question)
//	qb.WithSelect("users")
//	// Generates SQL like: SELECT * FROM users WHERE name = ?
//
//	qb := NewSqlBuilderWithPlaceholderFormat(ctx, squirrel.AtP)
//	qb.WithSelect("users")
//	// Generates SQL like: SELECT * FROM users WHERE name = @p1
func NewSqlBuilderWithPlaceholderFormat(ctx context.Context, placeholderFormat squirrel.PlaceholderFormat) *SqlBuilder {
	return &SqlBuilder{
		ctx:               ctx,
		placeholderFormat: placeholderFormat,
	}
}

// WithSelect sets up the QueryBuilder for SELECT operations
func (qb *SqlBuilder) WithSelect(table string) *SqlBuilder {
	psql := squirrel.StatementBuilder.PlaceholderFormat(qb.placeholderFormat)
	qb.selectBuilder = psql.Select("*").From(table)
	qb.queryType = selectQuery
	return qb
}

// WithUpdate sets up the QueryBuilder for UPDATE operations
func (qb *SqlBuilder) WithUpdate(table string) *SqlBuilder {
	psql := squirrel.StatementBuilder.PlaceholderFormat(qb.placeholderFormat)
	qb.updateBuilder = psql.Update(table)
	qb.queryType = updateQuery
	return qb
}

// WithDelete sets up the QueryBuilder for DELETE operations
func (qb *SqlBuilder) WithDelete(table string) *SqlBuilder {
	psql := squirrel.StatementBuilder.PlaceholderFormat(qb.placeholderFormat)
	qb.deleteBuilder = psql.Delete(table)
	qb.queryType = deleteQuery
	return qb
}

// WithInsert sets up the QueryBuilder for INSERT operations
func (qb *SqlBuilder) WithInsert(table string) *SqlBuilder {
	psql := squirrel.StatementBuilder.PlaceholderFormat(qb.placeholderFormat)
	qb.insertBuilder = psql.Insert(table)
	qb.queryType = insertQuery
	return qb
}
