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
		var subQuery elastic.Query
		switch filter.Operator {
		case OpEq:
			subQuery = elastic.NewTermQuery(filter.Field, filter.Value)
		case OpNe:
			subQuery = elastic.NewBoolQuery().MustNot(elastic.NewTermQuery(filter.Field, filter.Value))
		case OpLt:
			subQuery = elastic.NewRangeQuery(filter.Field).Lt(filter.Value)
		case OpLte:
			subQuery = elastic.NewRangeQuery(filter.Field).Lte(filter.Value)
		case OpGt:
			subQuery = elastic.NewRangeQuery(filter.Field).Gt(filter.Value)
		case OpGte:
			subQuery = elastic.NewRangeQuery(filter.Field).Gte(filter.Value)
		case OpIn:
			subQuery = elastic.NewTermsQuery(filter.Field, filter.Value.([]any)...)
		case OpNin:
			subQuery = elastic.NewBoolQuery().MustNot(elastic.NewTermsQuery(filter.Field, filter.Value.([]any)...))
		case OpAnd:
			subQuery = elastic.NewBoolQuery().Must(elastic.NewTermQuery(filter.Field, filter.Value))
		case OpOr:
			subQuery = elastic.NewBoolQuery().Should(elastic.NewTermQuery(filter.Field, filter.Value))
		}
		q.Must(subQuery)
	}

	return q, nil
}
