package queryparser

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Example struct for testing
type ScoringEvent struct {
	PlayerID      string    `json:"player_id" db:"player_id"`
	State         string    `json:"state" db:"state"`
	ActivatedAt   time.Time `json:"activated_at" db:"activated_at"`
	DeactivatedAt time.Time `json:"deactivated_at" db:"deactivated_at"`
	EventTime     time.Time `json:"event_time" db:"event_time"`
}

func TestProgrammaticOrFilter(t *testing.T) {
	ctx := context.Background()
	eventTime := time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC)

	// Build filters programmatically as shown in the user's example
	filters := []Filter{
		{
			Field:    "player_id",
			Operator: OpEq,
			Value:    "player123",
		},
		{
			Field:    "state",
			Operator: OpEq,
			Value:    "active",
		},
		{
			Field:    "",
			Operator: OpOr,
			Filters: []Filter{
				{
					Field:    "activated_at",
					Operator: OpLte,
					Value:    eventTime,
				},
				{
					Field:    "deactivated_at",
					Operator: OpGte,
					Value:    eventTime,
				},
			},
		},
	}

	// Create query builder
	qb := NewSqlBuilder(ctx)
	qb.WithSelect("scoring_events")

	// Apply filters
	model := ScoringEvent{}
	result, err := qb.Apply(filters, nil, model)
	assert.NoError(t, err)

	// Generate SQL
	sql, args, err := result.ToSql()
	assert.NoError(t, err)

	// Verify SQL structure
	t.Logf("Generated SQL: %s", sql)
	t.Logf("Generated Args: %v", args)

	// Check that the SQL contains expected conditions
	assert.Contains(t, sql, "player_id = $1")
	assert.Contains(t, sql, "state = $2")
	assert.Contains(t, sql, "activated_at <= $3")
	assert.Contains(t, sql, "deactivated_at >= $4")

	// Verify arguments
	assert.Len(t, args, 4)
	assert.Equal(t, "player123", args[0])
	assert.Equal(t, "active", args[1])
	assert.Equal(t, eventTime, args[2])
	assert.Equal(t, eventTime, args[3])
}

func TestNestedOrAndFilter(t *testing.T) {
	ctx := context.Background()
	eventTime := time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC)

	// More complex example with nested AND inside OR
	filters := []Filter{
		{
			Field:    "player_id",
			Operator: OpEq,
			Value:    "player123",
		},
		{
			Field:    "",
			Operator: OpOr,
			Filters: []Filter{
				{
					Field:    "state",
					Operator: OpEq,
					Value:    "active",
				},
				{
					Field:    "",
					Operator: OpAnd,
					Filters: []Filter{
						{
							Field:    "state",
							Operator: OpEq,
							Value:    "inactive",
						},
						{
							Field:    "activated_at",
							Operator: OpLte,
							Value:    eventTime,
						},
						{
							Field:    "deactivated_at",
							Operator: OpGte,
							Value:    eventTime,
						},
					},
				},
			},
		},
	}

	// Create query builder
	qb := NewSqlBuilder(ctx)
	qb.WithSelect("scoring_events")

	// Apply filters
	model := ScoringEvent{}
	result, err := qb.Apply(filters, nil, model)
	assert.NoError(t, err)

	// Generate SQL
	sql, args, err := result.ToSql()
	assert.NoError(t, err)

	// Verify SQL structure
	t.Logf("Generated SQL: %s", sql)
	t.Logf("Generated Args: %v", args)

	// Check that the SQL contains expected conditions
	assert.Contains(t, sql, "player_id = $1")
	assert.Contains(t, sql, "state = $2")
	assert.Contains(t, sql, "state = $3")
	assert.Contains(t, sql, "activated_at <= $4")
	assert.Contains(t, sql, "deactivated_at >= $5")

	// Verify arguments
	assert.Len(t, args, 5)
	assert.Equal(t, "player123", args[0])
}
