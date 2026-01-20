# Query Execution Patterns

This document covers query execution patterns for the pgEdge Postgres MCP
Server.

## Read-Only Execution

The MCP server enforces read-only query execution by default to prevent
accidental data modification.

### Transaction Mode

```go
tx, err := pool.BeginTx(ctx, pgx.TxOptions{
    AccessMode: pgx.ReadOnly,
})
if err != nil {
    return err
}
defer tx.Rollback(ctx)

rows, err := tx.Query(ctx, sql)
// Process rows...

err = tx.Commit(ctx)
```

### Read-Only Benefits

- Prevents accidental writes
- Can use read replicas
- Clearer intent in code
- Better error messages on write attempts

## Query Timeout Handling

All queries should have timeouts to prevent runaway queries.

### Context Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

rows, err := pool.Query(ctx, sql)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return fmt.Errorf("query timed out after 30s")
    }
    return err
}
```

### Statement Timeout

Set at connection level:

```go
config.ConnConfig.RuntimeParams["statement_timeout"] = "30000"  // 30 seconds
```

Or per query:

```sql
SET LOCAL statement_timeout = '30s';
SELECT ...;
```

## Parameter Binding

Always use parameterized queries to prevent SQL injection.

### Positional Parameters

```go
rows, err := pool.Query(ctx,
    "SELECT * FROM users WHERE id = $1 AND status = $2",
    userID, "active")
```

### Named Parameters (pgx)

```go
args := pgx.NamedArgs{
    "id":     userID,
    "status": "active",
}
rows, err := pool.Query(ctx,
    "SELECT * FROM users WHERE id = @id AND status = @status",
    args)
```

## Result Processing

### Row Iteration

```go
rows, err := pool.Query(ctx, sql)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    // Process row...
}

if err := rows.Err(); err != nil {
    return err
}
```

### Single Row

```go
var count int
err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        // Handle no rows case
    }
    return err
}
```

### Collecting All Rows

```go
rows, err := pool.Query(ctx, sql)
if err != nil {
    return nil, err
}
defer rows.Close()

results, err := pgx.CollectRows(rows, pgx.RowToMap)
if err != nil {
    return nil, err
}
```

## EXPLAIN Analysis

The MCP server provides query analysis through EXPLAIN.

### Basic EXPLAIN

```go
rows, err := pool.Query(ctx, "EXPLAIN " + sql)
```

### EXPLAIN ANALYZE

```go
// Only in read-only mode for SELECT queries
rows, err := pool.Query(ctx, "EXPLAIN ANALYZE " + sql)
```

### EXPLAIN Options

```sql
EXPLAIN (FORMAT JSON, ANALYZE, BUFFERS, TIMING) SELECT ...;
```

Options:

- `FORMAT`: TEXT, JSON, XML, YAML
- `ANALYZE`: Execute and show actual times
- `BUFFERS`: Show buffer usage
- `TIMING`: Show actual timing (can be disabled for less overhead)
- `COSTS`: Show estimated costs (default on)

## Error Handling

### Query Errors

```go
rows, err := pool.Query(ctx, sql)
if err != nil {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "42P01":
            return fmt.Errorf("table not found: %s", pgErr.Message)
        case "42703":
            return fmt.Errorf("column not found: %s", pgErr.Message)
        case "42601":
            return fmt.Errorf("syntax error: %s", pgErr.Message)
        default:
            return fmt.Errorf("query error: %s", pgErr.Message)
        }
    }
    return err
}
```

### Common Error Codes

| Code | Meaning |
|------|---------|
| 42601 | Syntax error |
| 42P01 | Undefined table |
| 42703 | Undefined column |
| 42883 | Undefined function |
| 22P02 | Invalid text representation |
| 25006 | Read-only transaction |
| 57014 | Query canceled (timeout) |

## Query Logging

For debugging and audit purposes:

```go
// Log query with timing
start := time.Now()
rows, err := pool.Query(ctx, sql)
duration := time.Since(start)

logging.Info("query executed",
    "sql", sql,
    "duration_ms", duration.Milliseconds(),
    "error", err)
```

## Best Practices

### Do

- Use parameterized queries
- Set query timeouts
- Use read-only transactions for SELECT
- Close rows when done
- Check rows.Err() after iteration

### Don't

- Concatenate user input into SQL
- Run queries without timeout
- Ignore query errors
- Leave rows open
- Use EXPLAIN ANALYZE on write queries in production
