package queryparser

import (
	"encoding/json"
	"testing"

	"github.com/olivere/elastic/v7"
)

func TestElasticBuilder(t *testing.T) {
	ss := elastic.NewSearchService(nil)

	tests := []struct {
		name    string
		filters []Filter
		want    string
		wantErr bool
	}{
		{
			name: "equals filter",
			filters: []Filter{
				{Field: "age", Operator: OpEq, Value: 25},
			},
			want:    `{"bool":{"must":{"term":{"age":25}}}}`,
			wantErr: false,
		},
		{
			name: "not equals filter",
			filters: []Filter{
				{Field: "age", Operator: OpNe, Value: 25},
			},
			want:    `{"bool":{"must":{"bool":{"must_not":{"term":{"age":25}}}}}}`,
			wantErr: false,
		},
		{
			name: "less than filter",
			filters: []Filter{
				{Field: "age", Operator: OpLt, Value: 25},
			},
			want:    `{"bool":{"must":{"range":{"age":{"from":null,"include_lower":true,"include_upper":false,"to":25}}}}}`,
			wantErr: false,
		},
		{
			name: "less than or equal filter",
			filters: []Filter{
				{Field: "age", Operator: OpLte, Value: 25},
			},
			want:    `{"bool":{"must":{"range":{"age":{"from":null,"include_lower":true,"include_upper":true,"to":25}}}}}`,
			wantErr: false,
		},
		{
			name: "greater than filter",
			filters: []Filter{
				{Field: "age", Operator: OpGt, Value: 25},
			},
			want:    `{"bool":{"must":{"range":{"age":{"from":25,"include_lower":false,"include_upper":true,"to":null}}}}}`,
			wantErr: false,
		},
		{
			name: "greater than or equal filter",
			filters: []Filter{
				{Field: "age", Operator: OpGte, Value: 25},
			},
			want:    `{"bool":{"must":{"range":{"age":{"from":25,"include_lower":true,"include_upper":true,"to":null}}}}}`,
			wantErr: false,
		},
		{
			name: "in filter",
			filters: []Filter{
				{Field: "age", Operator: OpIn, Value: []interface{}{25, 30, 35}},
			},
			want:    `{"bool":{"must":{"terms":{"age":[25,30,35]}}}}`,
			wantErr: false,
		},
		{
			name: "not in filter",
			filters: []Filter{
				{Field: "age", Operator: OpNin, Value: []interface{}{25, 30, 35}},
			},
			want:    `{"bool":{"must":{"bool":{"must_not":{"terms":{"age":[25,30,35]}}}}}}`,
			wantErr: false,
		},
		{
			name: "and filter",
			filters: []Filter{
				{Field: "age", Operator: OpAnd, Value: 25},
			},
			want:    `{"bool":{"must":{"bool":{"must":{"term":{"age":25}}}}}}`,
			wantErr: false,
		},
		{
			name: "or filter",
			filters: []Filter{
				{Field: "age", Operator: OpOr, Value: 25},
			},
			want:    `{"bool":{"must":{"bool":{"should":{"term":{"age":25}}}}}}`,
			wantErr: false,
		},
		{
			name: "multiple filters",
			filters: []Filter{
				{Field: "age", Operator: OpGt, Value: 25},
				{Field: "name", Operator: OpEq, Value: "John"},
			},
			want:    `{"bool":{"must":[{"range":{"age":{"from":25,"include_lower":false,"include_upper":true,"to":null}}},{"term":{"name":"John"}}]}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := NewElasticBuilder(ss)
			got, err := eb.Apply(tt.filters, nil, &TestUser{})

			if (err != nil) != tt.wantErr {
				t.Errorf("ElasticBuilder.Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				source, err := got.Source()
				if err != nil {
					t.Errorf("Error getting query source: %v", err)
					return
				}

				sourceStr, err := json.Marshal(source)
				if err != nil {
					t.Errorf("Error marshaling query source: %v", err)
					return
				}

				if string(sourceStr) != tt.want {
					t.Errorf("want %v; got %v", tt.want, string(sourceStr))
				}
			}
		})
	}
}
