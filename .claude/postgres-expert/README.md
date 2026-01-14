# PostgreSQL Expert Knowledge Base

This directory contains documentation to support expert-level PostgreSQL
guidance for the pgEdge Postgres MCP Server project.

## Purpose

The postgres-expert agent provides guidance on:

- PostgreSQL administration and troubleshooting
- Query optimization and performance tuning
- Database configuration best practices
- Connection management patterns
- Security and access control

## Project Context

The pgEdge Postgres MCP Server is a natural language interface for PostgreSQL.
It connects to user-managed PostgreSQL databases and executes queries on their
behalf. The server itself does not maintain its own database schema.

### Key Characteristics

- **No internal database**: Authentication uses YAML configuration files
- **User database connections**: Connects to external PostgreSQL instances
- **Read-only by default**: Query execution uses read-only transactions
- **pgx/v5**: Uses the pgx driver for PostgreSQL connections
- **Connection pooling**: Manages connection pools per database

## Documentation Files

### [connection-patterns.md](connection-patterns.md)

Connection management patterns used in the project:

- pgx/v5 connection pool configuration
- Connection string handling
- SSL/TLS configuration
- Connection lifecycle management
- Error handling patterns

### [query-execution.md](query-execution.md)

Query execution best practices:

- Read-only transaction enforcement
- Query timeout handling
- Result set processing
- Parameter binding
- Error handling

### [performance-tuning.md](performance-tuning.md)

PostgreSQL performance guidance:

- Index design principles
- Query optimization techniques
- EXPLAIN ANALYZE interpretation
- Common performance issues
- Configuration tuning

### [security-best-practices.md](security-best-practices.md)

Security guidance for PostgreSQL:

- Connection security (SSL/TLS)
- Authentication methods
- Role and privilege management
- SQL injection prevention
- Audit logging

## Quick Reference

### Connection String Format

```
postgresql://user:password@host:port/database?sslmode=require
```

### Common pgx Pool Settings

```go
config.MaxConns = 10
config.MinConns = 2
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = 30 * time.Minute
```

### Read-Only Transaction

```go
tx, err := pool.BeginTx(ctx, pgx.TxOptions{
    AccessMode: pgx.ReadOnly,
})
```

### Query with Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
rows, err := pool.Query(ctx, sql, args...)
```

## Related Project Files

- `/internal/database/` - Database client implementation
- `/internal/tools/query_database.go` - Query execution tool
- `/internal/tools/execute_explain.go` - EXPLAIN tool
- `/internal/config/` - Configuration loading

## PostgreSQL Version Support

The MCP server supports PostgreSQL 12 and later. Some features may require
newer versions:

- PostgreSQL 12+: Basic functionality
- PostgreSQL 13+: Enhanced statistics views
- PostgreSQL 14+: Query ID in pg_stat_statements
- PostgreSQL 15+: MERGE statement support

## External Resources

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [pgx Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [PostgreSQL Wiki](https://wiki.postgresql.org/)
