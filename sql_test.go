package queryparser

import (
	"context"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
)

// TestUser represents a user model with JSON tags
type TestUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Age       int    `json:"age"`
	Email     string `json:"email"`
	Password  string `json:"-"` // Private field, should not be filterable
	CreatedAt string `json:"created_at"`
}

// TestUserNoTags represents a user model without JSON tags
type TestUserNoTags struct {
	ID   int
	Name string
	Age  int
}

func TestToSql(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		builder  func() *SqlBuilder
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}{
		{
			name: "select query",
			builder: func() *SqlBuilder {
				qb := NewSqlBuilder(ctx)
				qb.WithSelect("users")
				return qb
			},
			wantSQL:  "SELECT * FROM users",
			wantArgs: nil,
			wantErr:  false,
		},
		{
			name: "select with where clause",
			builder: func() *SqlBuilder {
				qb := NewSqlBuilder(ctx)
				qb.WithSelect("users")
				filters := []Filter{
					{Field: "age", Operator: OpGt, Value: 18},
				}
				qb.Apply(filters, nil, &TestUser{})
				return qb
			},
			wantSQL:  "SELECT * FROM users WHERE (age > $1)",
			wantArgs: []any{18},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb := tt.builder()
			sql, args, err := qb.ToSql()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantSQL, sql)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantLen  int
		validate func(t *testing.T, filters []Filter)
	}{
		{
			name:    "empty filter",
			input:   "{}",
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "simple equality",
			input:   `{"age": 20}`,
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, filters []Filter) {
				assert.Equal(t, "age", filters[0].Field)
				assert.Equal(t, OpEq, filters[0].Operator)
				assert.Equal(t, float64(20), filters[0].Value)
			},
		},
		{
			name:    "multiple fields",
			input:   `{"age": 20, "name": "mike"}`,
			wantErr: false,
			wantLen: 2,
			validate: func(t *testing.T, filters []Filter) {
				// Create a map of fields for easier validation
				fields := make(map[string]bool)
				for _, f := range filters {
					fields[f.Field] = true
				}
				assert.True(t, fields["age"], "should have age field")
				assert.True(t, fields["name"], "should have name field")
			},
		},
		{
			name:    "operator $gt",
			input:   `{"age": {"$gt": 20}}`,
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, filters []Filter) {
				assert.Equal(t, "age", filters[0].Field)
				assert.Equal(t, OpGt, filters[0].Operator)
				assert.Equal(t, float64(20), filters[0].Value)
			},
		},
		{
			name:    "operator $in",
			input:   `{"age": {"$in": [20, 1]}}`,
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, filters []Filter) {
				assert.Equal(t, "age", filters[0].Field)
				assert.Equal(t, OpIn, filters[0].Operator)
				assert.Equal(t, []any{float64(20), float64(1)}, filters[0].Value)
			},
		},
		{
			name:    "operator $or",
			input:   `{"$or": [{"age": {"$gt": 20}}, {"name": "mike"}]}`,
			wantErr: false,
			wantLen: 2,
			validate: func(t *testing.T, filters []Filter) {
				// Create maps for easier validation
				fields := make(map[string]bool)
				operators := make(map[string]Operator)
				for _, f := range filters {
					fields[f.Field] = true
					operators[f.Field] = f.Operator
				}
				assert.True(t, fields["age"], "should have age field")
				assert.True(t, fields["name"], "should have name field")
				assert.Equal(t, OpGt, operators["age"])
				assert.Equal(t, OpEq, operators["name"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, err := ParseFilter(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, filters, tt.wantLen)
			if tt.validate != nil {
				tt.validate(t, filters)
			}
		})
	}
}

func TestParseQueryOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(t *testing.T, options *QueryOptions)
	}{
		{
			name:    "empty options",
			input:   "",
			wantErr: false,
			validate: func(t *testing.T, options *QueryOptions) {
				assert.Empty(t, options.Sort)
				assert.Nil(t, options.Limit)
				assert.Nil(t, options.Offset)
			},
		},
		{
			name:    "sort only",
			input:   `{"sort": {"age": "desc", "name": "asc"}}`,
			wantErr: false,
			validate: func(t *testing.T, options *QueryOptions) {
				assert.Equal(t, SortDesc, options.Sort["age"])
				assert.Equal(t, SortAsc, options.Sort["name"])
			},
		},
		{
			name:    "pagination only",
			input:   `{"limit": 10, "offset": 20}`,
			wantErr: false,
			validate: func(t *testing.T, options *QueryOptions) {
				assert.Equal(t, 10, *options.Limit)
				assert.Equal(t, 20, *options.Offset)
			},
		},
		{
			name:    "sort and pagination",
			input:   `{"sort": {"age": "desc"}, "limit": 10, "offset": 20}`,
			wantErr: false,
			validate: func(t *testing.T, options *QueryOptions) {
				assert.Equal(t, SortDesc, options.Sort["age"])
				assert.Equal(t, 10, *options.Limit)
				assert.Equal(t, 20, *options.Offset)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, err := ParseQueryOptions(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, options)
			}
		})
	}
}

func TestQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		filters  []Filter
		options  *QueryOptions
		model    any
		wantErr  bool
		validate func(t *testing.T, qb *SqlBuilder)
	}{
		{
			name: "valid fields with JSON tags",
			filters: []Filter{
				{Field: "age", Operator: OpEq, Value: 20},
				{Field: "name", Operator: OpEq, Value: "mike"},
			},
			model: &TestUser{},
			validate: func(t *testing.T, qb *SqlBuilder) {
				sql, args, err := qb.selectBuilder.ToSql()
				assert.NoError(t, err)
				assert.Contains(t, sql, "WHERE (age = $1 AND name = $2)")
				assert.Equal(t, []any{20, "mike"}, args)
			},
		},
		{
			name: "valid sort fields with JSON tags",
			filters: []Filter{
				{Field: "age", Operator: OpGt, Value: 20},
			},
			options: &QueryOptions{
				Sort: map[string]SortDirection{
					"age":  SortDesc,
					"name": SortAsc,
				},
			},
			model: &TestUser{},
			validate: func(t *testing.T, qb *SqlBuilder) {
				sql, args, err := qb.selectBuilder.ToSql()
				assert.NoError(t, err)
				assert.Contains(t, sql, "WHERE (age > $1)")
				assert.Contains(t, sql, "ORDER BY age DESC, name ASC")
				assert.Equal(t, []any{20}, args)
			},
		},
		{
			name: "invalid field without JSON tag",
			filters: []Filter{
				{Field: "password", Operator: OpEq, Value: "secret"},
			},
			model:   &TestUser{},
			wantErr: true,
		},
		{
			name: "invalid sort field without JSON tag",
			filters: []Filter{
				{Field: "age", Operator: OpGt, Value: 20},
			},
			options: &QueryOptions{
				Sort: map[string]SortDirection{
					"password": SortDesc,
				},
			},
			model:   &TestUser{},
			wantErr: true,
		},
		{
			name: "struct without JSON tags",
			filters: []Filter{
				{Field: "age", Operator: OpGt, Value: 20},
			},
			model:   &TestUserNoTags{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb := NewSqlBuilder(context.Background()).WithSelect("users")
			qb, err := qb.Apply(tt.filters, tt.options, tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, qb)
			}
		})
	}
}

func TestPlaceholderFormat(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		placeholderFormat squirrel.PlaceholderFormat
		expectedSQL       string
		expectedArgs      []any
	}{
		{
			name:              "dollar placeholder format",
			placeholderFormat: squirrel.Dollar,
			expectedSQL:       "SELECT * FROM users WHERE (name = $1)",
			expectedArgs:      []any{"John"},
		},
		{
			name:              "question placeholder format",
			placeholderFormat: squirrel.Question,
			expectedSQL:       "SELECT * FROM users WHERE (name = ?)",
			expectedArgs:      []any{"John"},
		},
		{
			name:              "at placeholder format",
			placeholderFormat: squirrel.AtP,
			expectedSQL:       "SELECT * FROM users WHERE (name = @p1)",
			expectedArgs:      []any{"John"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test NewSqlBuilderWithPlaceholderFormat
			qb := NewSqlBuilderWithPlaceholderFormat(ctx, tt.placeholderFormat)
			assert.Equal(t, tt.placeholderFormat, qb.GetPlaceholderFormat())

			qb.WithSelect("users")
			filters := []Filter{
				{Field: "name", Operator: OpEq, Value: "John"},
			}
			user := TestUser{}

			qb, err := qb.Apply(filters, nil, user)
			assert.NoError(t, err)

			sql, args, err := qb.ToSql()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestSetPlaceholderFormat(t *testing.T) {
	ctx := context.Background()

	// Test changing placeholder format after creation
	qb := NewSqlBuilder(ctx)
	assert.Equal(t, squirrel.Dollar, qb.GetPlaceholderFormat())

	qb.SetPlaceholderFormat(squirrel.Question)
	assert.Equal(t, squirrel.Question, qb.GetPlaceholderFormat())

	qb.WithSelect("users")
	filters := []Filter{
		{Field: "name", Operator: OpEq, Value: "John"},
	}
	user := TestUser{}

	qb, err := qb.Apply(filters, nil, user)
	assert.NoError(t, err)

	sql, args, err := qb.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "SELECT * FROM users WHERE (name = ?)", sql)
	assert.Equal(t, []any{"John"}, args)
}
