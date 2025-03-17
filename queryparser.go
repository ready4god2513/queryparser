package queryparser

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/Masterminds/squirrel"
)

// Operator represents MongoDB-style operators
type Operator string

const (
	OpEq  Operator = "$eq"
	OpNe  Operator = "$ne"
	OpLt  Operator = "$lt"
	OpLte Operator = "$lte"
	OpGt  Operator = "$gt"
	OpGte Operator = "$gte"
	OpIn  Operator = "$in"
	OpNin Operator = "$nin"
	OpAnd Operator = "$and"
	OpOr  Operator = "$or"
)

// Filter represents a MongoDB-style filter
type Filter struct {
	Field    string
	Operator Operator
	Value    interface{}
}

// SortDirection represents the direction of sorting
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

const (
	selectQuery int64 = iota
	updateQuery
	deleteQuery
	insertQuery
)

// QueryOptions represents additional query options like sorting and pagination
type QueryOptions struct {
	Sort   map[string]SortDirection `json:"sort,omitempty"`
	Limit  *int                     `json:"limit,omitempty"`
	Offset *int                     `json:"offset,omitempty"`
}

// QueryBuilder wraps Squirrel query builders and provides methods to apply filters
type QueryBuilder struct {
	queryType     int64
	selectBuilder squirrel.SelectBuilder
	updateBuilder squirrel.UpdateBuilder
	deleteBuilder squirrel.DeleteBuilder
	insertBuilder squirrel.InsertBuilder
	ctx           context.Context
}

// ToSql returns the SQL query string and arguments from the underlying Squirrel builder
func (qb *QueryBuilder) ToSql() (string, []interface{}, error) {
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

// ParseFilter parses a JSON string into a Filter
func ParseFilter(jsonStr string) ([]Filter, error) {
	var rawFilter map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &rawFilter); err != nil {
		return nil, fmt.Errorf("failed to parse filter JSON: %w", err)
	}

	return parseFilters(rawFilter)
}

// parseFilters recursively parses the filter map into Filter structs
func parseFilters(filter map[string]interface{}) ([]Filter, error) {
	var filters []Filter

	// Handle special operators first
	if orFilters, ok := filter[string(OpOr)].([]interface{}); ok {
		for _, f := range orFilters {
			if subFilter, ok := f.(map[string]interface{}); ok {
				subFilters, err := parseFilters(subFilter)
				if err != nil {
					return nil, err
				}
				filters = append(filters, subFilters...)
			}
		}
		return filters, nil
	}

	// Handle regular field filters
	for field, value := range filter {
		if field == string(OpOr) || field == string(OpAnd) {
			continue
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// Handle operators like $eq, $gt, etc.
			for op, val := range v {
				operator := Operator(op)
				filters = append(filters, Filter{
					Field:    field,
					Operator: operator,
					Value:    val,
				})
			}
		default:
			// Implicit $eq operator
			filters = append(filters, Filter{
				Field:    field,
				Operator: OpEq,
				Value:    v,
			})
		}
	}

	return filters, nil
}

// ParseQueryOptions parses a JSON string into QueryOptions
func ParseQueryOptions(jsonStr string) (*QueryOptions, error) {
	if jsonStr == "" {
		return &QueryOptions{}, nil
	}

	var options QueryOptions
	if err := json.Unmarshal([]byte(jsonStr), &options); err != nil {
		return nil, fmt.Errorf("failed to parse query options JSON: %w", err)
	}

	return &options, nil
}

// getJSONTags returns a map of field names to their JSON tags
func getJSONTags(v interface{}) (map[string]string, error) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct or pointer to struct, got %v", val.Kind())
	}

	tags := make(map[string]string)
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" {
			continue
		}
		// Handle json tag with options (e.g., "name,omitempty")
		parts := strings.Split(tag, ",")
		jsonName := parts[0]
		if jsonName == "-" {
			continue
		}
		tags[field.Name] = jsonName
	}
	return tags, nil
}

// validateFields validates that all fields in filters and options exist in the struct's JSON tags
func validateFields(filters []Filter, options *QueryOptions, tags map[string]string) error {
	// Validate filter fields
	for _, filter := range filters {
		// Check if the field exists in the JSON tags
		found := false
		for _, jsonTag := range tags {
			if jsonTag == filter.Field {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field %q is not a valid JSON field", filter.Field)
		}
	}

	// Validate sort fields
	if options != nil && len(options.Sort) > 0 {
		for field := range options.Sort {
			found := false
			for _, jsonTag := range tags {
				if jsonTag == field {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("field %q is not a valid JSON field for sorting", field)
			}
		}
	}

	return nil
}

// Apply applies the filters and options to the QueryBuilder
func (qb *QueryBuilder) Apply(filters []Filter, options *QueryOptions, model interface{}) (*QueryBuilder, error) {
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

// applySelectFilters applies filters to a SELECT query
func (qb *QueryBuilder) applySelectFilters(filters []Filter) (*QueryBuilder, error) {
	conditions := make([]squirrel.Sqlizer, 0, len(filters))

	for _, filter := range filters {
		condition, err := buildCondition(filter)
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
func buildCondition(filter Filter) (squirrel.Sqlizer, error) {
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
func (qb *QueryBuilder) applyOptions(options *QueryOptions) (*QueryBuilder, error) {
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

// NewQueryBuilder creates a new QueryBuilder instance
func NewQueryBuilder(ctx context.Context) *QueryBuilder {
	return &QueryBuilder{
		ctx: ctx,
	}
}

// WithSelect sets up the QueryBuilder for SELECT operations
func (qb *QueryBuilder) WithSelect(table string) *QueryBuilder {
	qb.selectBuilder = squirrel.Select("*").From(table)
	qb.queryType = selectQuery
	return qb
}

// WithUpdate sets up the QueryBuilder for UPDATE operations
func (qb *QueryBuilder) WithUpdate(table string) *QueryBuilder {
	qb.updateBuilder = squirrel.Update(table)
	qb.queryType = updateQuery
	return qb
}

// WithDelete sets up the QueryBuilder for DELETE operations
func (qb *QueryBuilder) WithDelete(table string) *QueryBuilder {
	qb.deleteBuilder = squirrel.Delete(table)
	qb.queryType = deleteQuery
	return qb
}

// WithInsert sets up the QueryBuilder for INSERT operations
func (qb *QueryBuilder) WithInsert(table string) *QueryBuilder {
	qb.insertBuilder = squirrel.Insert(table)
	qb.queryType = insertQuery
	return qb
}
