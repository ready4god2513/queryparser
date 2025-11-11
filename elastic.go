package queryparser

import (
	"github.com/olivere/elastic/v7"
)

type ElasticBuilder struct {
	ss *elastic.SearchService
}

func NewElasticBuilder(ss *elastic.SearchService) *ElasticBuilder {
	return &ElasticBuilder{ss: ss}
}

// Apply will create a bool query and apply the filters to it.  It will then
// return the query which can be used to execute the search.
func (eb *ElasticBuilder) Apply(filters []Filter, options *QueryOptions, model any) (elastic.Query, error) {
	q := elastic.NewBoolQuery()

	for _, filter := range filters {
		subQuery, err := eb.buildQuery(filter)
		if err != nil {
			return nil, err
		}
		q.Must(subQuery)
	}

	return q, nil
}

// buildQuery recursively builds elastic queries from filters
func (eb *ElasticBuilder) buildQuery(filter Filter) (elastic.Query, error) {
	// Handle $or operator with nested filters
	if filter.Operator == OpOr {
		orQuery := elastic.NewBoolQuery()
		for _, nestedFilter := range filter.Filters {
			subQuery, err := eb.buildQuery(nestedFilter)
			if err != nil {
				return nil, err
			}
			orQuery.Should(subQuery)
		}
		orQuery.MinimumNumberShouldMatch(1)
		return orQuery, nil
	}

	// Handle $and operator with nested filters
	if filter.Operator == OpAnd {
		andQuery := elastic.NewBoolQuery()
		for _, nestedFilter := range filter.Filters {
			subQuery, err := eb.buildQuery(nestedFilter)
			if err != nil {
				return nil, err
			}
			andQuery.Must(subQuery)
		}
		return andQuery, nil
	}

	// Handle regular operators
	switch filter.Operator {
	case OpEq:
		return elastic.NewTermQuery(filter.Field, filter.Value), nil
	case OpNe:
		return elastic.NewBoolQuery().MustNot(elastic.NewTermQuery(filter.Field, filter.Value)), nil
	case OpLt:
		return elastic.NewRangeQuery(filter.Field).Lt(filter.Value), nil
	case OpLte:
		return elastic.NewRangeQuery(filter.Field).Lte(filter.Value), nil
	case OpGt:
		return elastic.NewRangeQuery(filter.Field).Gt(filter.Value), nil
	case OpGte:
		return elastic.NewRangeQuery(filter.Field).Gte(filter.Value), nil
	case OpIn:
		return elastic.NewTermsQuery(filter.Field, filter.Value.([]any)...), nil
	case OpNin:
		return elastic.NewBoolQuery().MustNot(elastic.NewTermsQuery(filter.Field, filter.Value.([]any)...)), nil
	default:
		return nil, nil
	}
}
