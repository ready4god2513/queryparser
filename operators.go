package queryparser

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// SortDirection represents the direction of sorting
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// QueryOptions represents additional query options like sorting and pagination
type QueryOptions struct {
	Sort   map[string]SortDirection `json:"sort,omitempty"`
	Limit  *int                     `json:"limit,omitempty"`
	Offset *int                     `json:"offset,omitempty"`
}

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
