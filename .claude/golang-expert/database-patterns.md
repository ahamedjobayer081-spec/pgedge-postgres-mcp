/*-----------------------------------------------------------
 *
 * pgEdge Postgres MCP Server - Database Connection and Access Patterns
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-----------------------------------------------------------
 */

# Database Connection and Access Patterns

This document describes the database connection pooling, access patterns, and
best practices used in the pgEdge Postgres MCP Server.

## Connection Pooling with pgx/v5

The project uses `github.com/jackc/pgx/v5/pgxpool` for PostgreSQL connection
pooling. This is the recommended driver for Go PostgreSQL applications.

### Why pgx over database/sql?

1. **Better Performance:** Native PostgreSQL protocol, no CGO overhead
2. **Rich Feature Set:** Binary protocol, COPY, LISTEN/NOTIFY, batch queries
3. **Context Support:** First-class context.Context integration
4. **Type Safety:** Strong typing for PostgreSQL types
5. **Connection Pooling:** Built-in production-ready connection pool

### Basic Pool Creation

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

func Connect(connStr string) (*pgxpool.Pool, error) {
    poolConfig, err := pgxpool.ParseConfig(connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    // Configure pool settings
    poolConfig.MaxConns = 25
    poolConfig.MaxConnIdleTime = 5 * time.Minute
    poolConfig.HealthCheckPeriod = 1 * time.Minute

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create pool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return pool, nil
}
```

### Pool Configuration Options

```go
type PoolConfig struct {
    MaxConns        int32         // Maximum connections in pool (Default: 4)
    MinConns        int32         // Minimum idle connections (Default: 0)
    MaxConnIdleTime time.Duration // Max idle time (Default: 30 minutes)
    MaxConnLifetime time.Duration // Max connection lifetime (Default: 1 hour)
    HealthCheckPeriod time.Duration // Health check interval (Default: 1 minute)
}
```

**Recommended Settings:**

- MaxConns: 25 (handles concurrent requests)
- MaxConnIdleTime: 5 minutes
- HealthCheckPeriod: 1 minute

## Client Manager Pattern

The project manages multiple database connections through a ClientManager
located in `/internal/database/`:

```go
type ClientManager struct {
    clients map[string]*Client
    mu      sync.RWMutex
}

type Client struct {
    Name   string
    pool   *pgxpool.Pool
    config ConnectionConfig
}

func NewClientManager() *ClientManager {
    return &ClientManager{
        clients: make(map[string]*Client),
    }
}

func (m *ClientManager) AddConnection(name string, config ConnectionConfig) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    pool, err := createPool(config)
    if err != nil {
        return fmt.Errorf("failed to create pool: %w", err)
    }

    m.clients[name] = &Client{
        Name:   name,
        pool:   pool,
        config: config,
    }

    return nil
}

func (m *ClientManager) GetConnection(ctx context.Context, name string) (
    *pgxpool.Conn, error) {
    m.mu.RLock()
    client, exists := m.clients[name]
    m.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("connection not found: %s", name)
    }

    return client.pool.Acquire(ctx)
}
```

## Query Execution Patterns

### Simple Query

```go
func GetVersion(ctx context.Context, pool *pgxpool.Pool) (string, error) {
    var version string
    err := pool.QueryRow(ctx, "SELECT version()").Scan(&version)
    if err != nil {
        return "", fmt.Errorf("failed to query version: %w", err)
    }
    return version, nil
}
```

### Multiple Rows

```go
func GetTables(ctx context.Context, pool *pgxpool.Pool, schema string) (
    []string, error) {
    rows, err := pool.Query(ctx, `
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = $1
        ORDER BY table_name
    `, schema)
    if err != nil {
        return nil, fmt.Errorf("failed to query tables: %w", err)
    }
    defer rows.Close()

    tables := make([]string, 0)
    for rows.Next() {
        var table string
        if err := rows.Scan(&table); err != nil {
            return nil, fmt.Errorf("failed to scan table: %w", err)
        }
        tables = append(tables, table)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating tables: %w", err)
    }

    return tables, nil
}
```

### Transaction Pattern

```go
func ExecuteInTransaction(
    ctx context.Context,
    pool *pgxpool.Pool,
    fn func(tx pgx.Tx) error,
) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer func() {
        if err != nil {
            if rerr := tx.Rollback(ctx); rerr != nil {
                logging.Error("Rollback error", "error", rerr)
            }
        }
    }()

    if err = fn(tx); err != nil {
        return err
    }

    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}
```

**Transaction Best Practices:**

1. Always use `defer` to handle rollback
2. Keep transactions short to minimize lock contention
3. Use explicit `Commit()` to finalize
4. Log rollback errors but don't return them

## Connection String Building

```go
func BuildConnectionString(config ConnectionConfig) string {
    params := make(map[string]string)
    params["host"] = config.Host
    params["port"] = fmt.Sprintf("%d", config.Port)
    params["dbname"] = config.Database
    params["user"] = config.Username

    if config.Password != "" {
        params["password"] = config.Password
    }

    if config.SSLMode != "" {
        params["sslmode"] = config.SSLMode
    }

    params["application_name"] = "pgEdge Postgres MCP Server"

    var parts []string
    for key, value := range params {
        parts = append(parts, fmt.Sprintf("%s=%s", key, value))
    }
    return strings.Join(parts, " ")
}
```

## Connection Lifecycle Management

### Graceful Shutdown

```go
func (m *ClientManager) Close() {
    m.mu.Lock()
    defer m.mu.Unlock()

    for name, client := range m.clients {
        client.pool.Close()
        delete(m.clients, name)
    }
}
```

### Pool Health Monitoring

```go
func MonitorPoolStats(pool *pgxpool.Pool) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        stat := pool.Stat()
        logging.Info("Pool stats",
            "total", stat.TotalConns(),
            "idle", stat.IdleConns(),
            "acquired", stat.AcquiredConns())
    }
}
```

### Connection Timeout Handling

```go
func QueryWithTimeout(
    pool *pgxpool.Pool,
    query string,
    args ...interface{},
) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    _, err := pool.Exec(ctx, query, args...)
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return fmt.Errorf("query timed out after 5 seconds")
        }
        return fmt.Errorf("query failed: %w", err)
    }

    return nil
}
```

## Password Encryption

The project encrypts stored connection passwords using AES-256-GCM in
`/internal/crypto/`:

```go
func Encrypt(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(encrypted string, key []byte) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}
```

## Best Practices

1. **Always Use Contexts:** Pass context for cancellation and timeouts
2. **Release Connections:** Use `defer` to ensure connections are released
3. **Handle Errors Explicitly:** Check and wrap errors with context
4. **Use Transactions:** For operations modifying multiple tables
5. **Set Appropriate Timeouts:** Balance responsiveness vs. query complexity
6. **Monitor Pool Stats:** Track pool utilization in production
7. **Batch Operations:** Group INSERT/UPDATE operations for efficiency
8. **Close Rows:** Always call `rows.Close()` (or use `defer`)
9. **Check rows.Err():** After iteration to catch iteration errors

## Troubleshooting

### Connection Pool Exhaustion

**Symptoms:**

- Timeouts acquiring connections
- Slow API responses

**Solutions:**

- Increase MaxConns
- Check for connection leaks (missing Release calls)
- Add monitoring for pool stats

### Connection Timeouts

**Symptoms:**

- context deadline exceeded errors

**Solutions:**

- Increase timeout duration
- Optimize slow queries
- Add indexes

### Connection Leaks

**Symptoms:**

- Pool stats show high AcquiredConns that never decrease

**Solutions:**

- Audit code for missing `defer conn.Release()`
- Add connection tracking in development
- Review error handling paths
