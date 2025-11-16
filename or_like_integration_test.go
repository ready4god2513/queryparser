package queryparser

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOrLikeIntegration tests the full flow from JSON parsing to SQL generation
func TestOrLikeIntegration(t *testing.T) {
	type Player struct {
		Firstname string `json:"firstname" db:"firstname"`
		Lastname  string `json:"lastname" db:"lastname"`
		State     string `json:"state" db:"state"`
	}

	tests := []struct {
		name           string
		filterJSON     string
		shouldContain  string
		expectedParams int
	}{
		{
			name:           "OR with LIKE operators",
			filterJSON:     `{"$or": [{"firstname": {"$like": "Rom"}}, {"lastname": {"$like": "Rom"}}]}`,
			shouldContain:  "firstname LIKE $1 OR lastname LIKE $2",
			expectedParams: 2,
		},
		{
			name:           "OR with equality operators",
			filterJSON:     `{"$or": [{"firstname": "John"}, {"lastname": "Doe"}]}`,
			shouldContain:  "firstname = $1 OR lastname = $2",
			expectedParams: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse filter
			filters, err := ParseFilter(tt.filterJSON)
			assert.NoError(t, err)

			// Create SQL builder
			ctx := context.Background()
			qb := NewSqlBuilder(ctx)
			qb.WithSelect("players")

			// Apply filters
			model := Player{}
			result, err := qb.Apply(filters, nil, model)
			assert.NoError(t, err)

			// Generate SQL
			sql, args, err := result.ToSql()
			assert.NoError(t, err)

			// Log for debugging
			t.Logf("Generated SQL: %s", sql)
			t.Logf("Generated Args: %v", args)

			// Verify SQL structure contains the expected OR clause
			assert.Contains(t, sql, tt.shouldContain)
			assert.Len(t, args, tt.expectedParams)

			// Verify OR is present in SQL (not AND)
			if strings.Contains(tt.filterJSON, "$or") {
				assert.Contains(t, sql, " OR ", "SQL should contain OR operator")
			}

			// Verify LIKE parameters are correctly formatted
			for i, arg := range args {
				if str, ok := arg.(string); ok {
					if strings.Contains(tt.filterJSON, "$like") {
						assert.Contains(t, str, "%", "LIKE parameter %d should contain wildcard", i+1)
					}
				}
			}
		})
	}
}

// TestOrLikeBugFix verifies the specific bug reported is fixed
func TestOrLikeBugFix(t *testing.T) {
	type Player struct {
		Firstname string `json:"firstname" db:"firstname"`
		Lastname  string `json:"lastname" db:"lastname"`
		State     string `json:"state" db:"state"`
	}

	// The exact case from the bug report
	filterJSON := `{"$or": [{"firstname": {"$like": "Rom"}}, {"lastname": {"$like": "Rom"}}]}`

	filters, err := ParseFilter(filterJSON)
	assert.NoError(t, err)

	ctx := context.Background()
	qb := NewSqlBuilder(ctx)
	qb.WithSelect("players")

	model := Player{}
	result, err := qb.Apply(filters, nil, model)
	assert.NoError(t, err)

	sql, args, err := result.ToSql()
	assert.NoError(t, err)

	t.Logf("Generated SQL: %s", sql)
	t.Logf("Generated Args: %v", args)

	// The bug was: WHERE (firstname LIKE ? AND lastname LIKE ?)
	// Should be:   WHERE (firstname LIKE ? OR lastname LIKE ?)
	assert.Contains(t, sql, "firstname LIKE $1 OR lastname LIKE $2",
		"SQL should use OR, not AND between firstname and lastname")

	assert.NotContains(t, sql, "firstname LIKE $1 AND lastname LIKE $2",
		"SQL should NOT use AND between firstname and lastname")

	// Verify arguments
	assert.Len(t, args, 2)
	assert.Equal(t, "%Rom%", args[0])
	assert.Equal(t, "%Rom%", args[1])
}
