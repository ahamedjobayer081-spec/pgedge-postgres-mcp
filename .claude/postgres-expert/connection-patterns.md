# PostgreSQL Connection Patterns

This document covers connection management patterns for the pgEdge Postgres
MCP Server.

## Connection Architecture

The MCP server manages connections to user PostgreSQL databases through a
client manager that maintains connection pools per database.

### Key Components

- **ClientManager**: Manages multiple database clients
- **Client**: Wraps a pgx connection pool for a single database
- **DatabaseProvider**: Provides database connections to tools

## pgx/v5 Connection Pools

The project uses pgx/v5 for PostgreSQL connections with connection pooling.

### Pool Configuration

```go
config, err := pgxpool.ParseConfig(connectionString)
if err != nil {
    return nil, err
}

// Pool size settings
config.MaxConns = 10           // Maximum connections in pool
config.MinConns = 2            // Minimum idle connections
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = 30 * time.Minute

// Health check
config.HealthCheckPeriod = time.Minute

pool, err := pgxpool.NewWithConfig(ctx, config)
```

### Connection String Parameters

Standard PostgreSQL connection parameters:

```
postgresql://user:password@host:port/database?param=value
```

Common parameters:

- `sslmode`: disable, require, verify-ca, verify-full
- `connect_timeout`: Connection timeout in seconds
- `application_name`: Application identifier
- `search_path`: Schema search path

### SSL/TLS Configuration

For secure connections:

```go
config.ConnConfig.TLSConfig = &tls.Config{
    ServerName:         hostname,
    InsecureSkipVerify: false,  // Set true only for testing
}
```

SSL modes:

- `disable`: No SSL (not recommended for production)
- `require`: SSL required, no certificate verification
- `verify-ca`: Verify server certificate against CA
- `verify-full`: Verify certificate and hostname

## Connection Lifecycle

### Acquiring Connections

```go
// From pool (recommended)
conn, err := pool.Acquire(ctx)
if err != nil {
    return err
}
defer conn.Release()

// Direct query (pool manages connection)
rows, err := pool.Query(ctx, sql, args...)
```

### Connection Health

The pool automatically manages connection health:

- Validates connections before use
- Removes stale connections
- Maintains minimum idle connections

### Graceful Shutdown

```go
// Close all connections in pool
pool.Close()
```

## Error Handling

### Connection Errors

```go
if err != nil {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        // PostgreSQL-specific error
        log.Printf("PostgreSQL error: %s (code: %s)",
            pgErr.Message, pgErr.Code)
    }
    return err
}
```

### Common Error Codes

- `28P01`: Invalid password
- `3D000`: Database does not exist
- `42P01`: Undefined table
- `42703`: Undefined column
- `57P03`: Cannot connect now (server starting)

### Retry Logic

```go
func withRetry(ctx context.Context, fn func() error) error {
    var err error
    for i := 0; i < 3; i++ {
        err = fn()
        if err == nil {
            return nil
        }
        // Check if error is retryable
        if !isRetryable(err) {
            return err
        }
        time.Sleep(time.Second * time.Duration(i+1))
    }
    return err
}
```

## Connection Isolation

The MCP server maintains connection isolation between users:

- Each authenticated session gets its own database client
- Connections are not shared between users
- Connection pools are per-database, per-user

## Best Practices

### Do

- Use connection pools, not individual connections
- Set appropriate pool sizes for workload
- Use context with timeout for all queries
- Release connections promptly
- Handle connection errors gracefully

### Don't

- Hold connections longer than necessary
- Create new pools for every request
- Ignore connection errors
- Use unbounded pool sizes
- Skip SSL in production

## Monitoring Connections

### Pool Statistics

```go
stat := pool.Stat()
log.Printf("Total: %d, Idle: %d, Acquired: %d",
    stat.TotalConns(),
    stat.IdleConns(),
    stat.AcquiredConns())
```

### PostgreSQL Side

```sql
SELECT pid, usename, application_name, state, query_start
FROM pg_stat_activity
WHERE application_name = 'pgedge-mcp';
```
