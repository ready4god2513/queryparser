# Query Parser

A flexible and secure query parser for Go applications that converts MongoDB-style query filters into SQL queries using Squirrel. This package is designed to work with Echo and other Go web frameworks, providing a clean API for handling complex database queries.

## Features

- MongoDB-style query syntax for easy-to-use filtering
- JSON tag-based field validation for security
- Support for sorting and pagination
- Integration with Squirrel for SQL query building
- Type-safe query construction
- Protection against SQL injection

## Installation

```bash
go get github.com/ready4god2513/queryparser
```

## Usage

### Basic Setup

First, define your model struct with JSON tags:

```go
type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Age       int       `json:"age"`
    Email     string    `json:"email"`
    Password  string    `json:"-"` // Private field, not filterable
    CreatedAt time.Time `json:"created_at"`
}
```

### Using with Echo

```go
func ListUsers(c echo.Context) error {
    // Get the filter and options from query parameters
    filter := c.QueryParam("filter")
    options := c.QueryParam("options")

    // Parse the filter and options
    filters, err := queryparser.ParseFilter(filter)
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "Invalid filter format",
        })
    }

    queryOptions, err := queryparser.ParseQueryOptions(options)
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "Invalid options format",
        })
    }

    // Create a new query builder
    qb := queryparser.NewSqlBuilder(c.Request().Context()).WithSelect("users")

    // Apply the filters and options, passing the model for validation
    qb, err = qb.Apply(filters, queryOptions, &User{})
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": err.Error(),
        })
    }

    // Get the SQL and arguments
    sql, args, err := qb.selectBuilder.ToSql()
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": "Failed to build SQL",
        })
    }

    // Execute the query with your database
    rows, err := db.QueryContext(c.Request().Context(), sql, args...)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": "Failed to execute query",
        })
    }
    defer rows.Close()

    // Process the results...
    var users []User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Name, &user.Age, &user.Email, &user.Password, &user.CreatedAt); err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{
                "error": "Failed to scan row",
            })
        }
        users = append(users, user)
    }

    return c.JSON(http.StatusOK, users)
}
```

## Query Syntax

### Filtering

The filter syntax follows MongoDB's query syntax:

```json
{
  "field": "value", // Implicit $eq operator
  "age": { "$gt": 20 }, // Greater than
  "age": { "$lt": 20 }, // Less than
  "age": { "$gte": 20 }, // Greater than or equal
  "age": { "$lte": 20 }, // Less than or equal
  "age": { "$ne": 20 }, // Not equal
  "age": { "$in": [20, 30] }, // In array
  "age": { "$nin": [20, 30] } // Not in array
}
```

### Complex Queries

You can combine conditions using `$or`:

```json
{
  "$or": [{ "age": { "$gt": 20 } }, { "name": "mike" }]
}
```

### Sorting and Pagination

Use the `options` parameter to specify sorting and pagination:

```json
{
  "sort": {
    "age": "desc",
    "name": "asc"
  },
  "limit": 10,
  "offset": 20
}
```

## Placeholder Formats

The query builder supports different SQL placeholder formats to work with various databases:

### Using Default Format (Dollar - PostgreSQL)

```go
// Creates a query builder with default Dollar format ($1, $2, etc.)
qb := queryparser.NewSqlBuilder(ctx).WithSelect("users")
// Generates SQL like: SELECT * FROM users WHERE name = $1
```

### Using Question Format (MySQL/SQLite)

```go
// Creates a query builder with Question format (?, ?, etc.)
qb := queryparser.NewSqlBuilderWithPlaceholderFormat(ctx, squirrel.Question)
qb.WithSelect("users")
// Generates SQL like: SELECT * FROM users WHERE name = ?
```

### Using AtP Format (SQL Server)

```go
// Creates a query builder with AtP format (@p1, @p2, etc.)
qb := queryparser.NewSqlBuilderWithPlaceholderFormat(ctx, squirrel.AtP)
qb.WithSelect("users")
// Generates SQL like: SELECT * FROM users WHERE name = @p1
```

### Changing Format After Creation

```go
qb := queryparser.NewSqlBuilder(ctx)
qb.SetPlaceholderFormat(squirrel.Question)
qb.WithSelect("users")
// Now uses Question format instead of default Dollar format
```

### Getting Current Format

```go
qb := queryparser.NewSqlBuilder(ctx)
format := qb.GetPlaceholderFormat()
// Returns the current placeholder format
```

## Security Features

1. **JSON Tag Validation**: Only fields with JSON tags can be used in filters and sorting
2. **Private Fields**: Fields marked with `json:"-"` are not filterable
3. **SQL Injection Protection**: All queries are parameterized using Squirrel
4. **Type Safety**: The query builder ensures type-safe query construction

## Best Practices

1. **Model Definition**:

   - Always use JSON tags for your model fields
   - Mark private fields with `json:"-"`
   - Use descriptive field names that match your database columns

2. **Error Handling**:

   - Always check for errors when parsing filters and options
   - Return appropriate HTTP status codes for different error types
   - Provide clear error messages to API consumers

3. **Performance**:

   - Use appropriate indexes on your database columns
   - Limit the number of records returned using pagination
   - Consider using cursor-based pagination for large datasets

4. **API Design**:
   - Document the available filter fields and operators
   - Provide examples in your API documentation
   - Consider rate limiting for complex queries

## Example API Requests

1. Basic filtering:

```
GET /users?filter={"age":{"$gt":20},"name":"mike"}
```

2. Complex filtering with sorting:

```
GET /users?filter={"$or":[{"age":{"$gt":20}},{"name":"mike"}]}&options={"sort":{"age":"desc"}}
```

3. Pagination:

```
GET /users?options={"limit":10,"offset":0}
```

4. Combined filtering, sorting, and pagination:

```
GET /users?filter={"age":{"$gt":20}}&options={"sort":{"age":"desc"},"limit":10,"offset":20}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
