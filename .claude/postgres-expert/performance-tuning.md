# PostgreSQL Performance Tuning

This document provides PostgreSQL performance guidance for users of the pgEdge
Postgres MCP Server.

## Query Optimization

### Using EXPLAIN

The EXPLAIN command shows how PostgreSQL executes a query:

```sql
EXPLAIN SELECT * FROM orders WHERE customer_id = 123;
```

With actual execution statistics:

```sql
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM orders WHERE customer_id = 123;
```

### Reading EXPLAIN Output

Key metrics to examine:

- **Seq Scan**: Full table scan (may indicate missing index)
- **Index Scan**: Using an index (generally good)
- **Nested Loop**: Join method (watch for high row estimates)
- **Hash Join**: Join method for larger datasets
- **Sort**: May spill to disk if work_mem too low
- **Rows**: Estimated vs actual rows (large differences indicate stale stats)

### Common Performance Issues

**Sequential scans on large tables**:

```sql
-- Check for missing indexes
SELECT schemaname, tablename, seq_scan, idx_scan
FROM pg_stat_user_tables
WHERE seq_scan > idx_scan
ORDER BY seq_scan DESC;
```

**Slow queries**:

```sql
-- Find slow queries (requires pg_stat_statements)
SELECT query, calls, mean_exec_time, total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

## Index Design

### When to Add Indexes

Add indexes for:

- Columns in WHERE clauses
- Columns in JOIN conditions
- Columns in ORDER BY
- Foreign key columns

### Index Types

**B-tree** (default): Equality and range queries

```sql
CREATE INDEX idx_orders_customer ON orders(customer_id);
```

**Hash**: Equality only (rarely needed)

```sql
CREATE INDEX idx_orders_status ON orders USING hash(status);
```

**GIN**: Full-text search, arrays, JSONB

```sql
CREATE INDEX idx_docs_content ON documents USING gin(to_tsvector('english', content));
```

**GiST**: Geometric data, full-text search

```sql
CREATE INDEX idx_locations_point ON locations USING gist(point);
```

### Partial Indexes

Index only relevant rows:

```sql
CREATE INDEX idx_orders_pending ON orders(created_at)
WHERE status = 'pending';
```

### Composite Indexes

Order matters - put equality columns first:

```sql
-- Good for: WHERE status = 'active' AND created_at > '2024-01-01'
CREATE INDEX idx_orders_status_date ON orders(status, created_at);
```

## Configuration Tuning

### Memory Settings

```ini
# Shared memory for caching (25% of RAM)
shared_buffers = 4GB

# Memory for query operations
work_mem = 64MB

# Memory for maintenance operations
maintenance_work_mem = 512MB

# Planner's estimate of available cache
effective_cache_size = 12GB
```

### Connection Settings

```ini
# Maximum concurrent connections
max_connections = 100

# Connections reserved for superuser
superuser_reserved_connections = 3
```

### Checkpoint Settings

```ini
# Maximum WAL size before checkpoint
max_wal_size = 2GB

# Minimum WAL size to maintain
min_wal_size = 512MB

# Spread checkpoint I/O
checkpoint_completion_target = 0.9
```

### Autovacuum Settings

```ini
# Enable autovacuum
autovacuum = on

# Trigger vacuum when this fraction of rows changed
autovacuum_vacuum_scale_factor = 0.1

# Trigger analyze when this fraction of rows changed
autovacuum_analyze_scale_factor = 0.05
```

## Query Patterns

### Efficient Pagination

Avoid OFFSET for large datasets:

```sql
-- Instead of:
SELECT * FROM orders ORDER BY id LIMIT 10 OFFSET 10000;

-- Use keyset pagination:
SELECT * FROM orders WHERE id > 10000 ORDER BY id LIMIT 10;
```

### Batch Operations

Process large datasets in batches:

```sql
-- Delete in batches
DELETE FROM logs
WHERE id IN (
    SELECT id FROM logs
    WHERE created_at < NOW() - INTERVAL '90 days'
    LIMIT 1000
);
```

### Avoiding N+1 Queries

Use JOINs instead of multiple queries:

```sql
-- Instead of querying orders then customers separately:
SELECT o.*, c.name as customer_name
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.status = 'pending';
```

## Monitoring

### Key Metrics

```sql
-- Cache hit ratio (should be > 99%)
SELECT
    sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as ratio
FROM pg_statio_user_tables;

-- Index usage
SELECT
    schemaname, tablename,
    idx_scan, seq_scan,
    idx_scan::float / (idx_scan + seq_scan) as idx_ratio
FROM pg_stat_user_tables
WHERE (idx_scan + seq_scan) > 0;

-- Table bloat indicator
SELECT
    schemaname, tablename,
    n_live_tup, n_dead_tup,
    n_dead_tup::float / (n_live_tup + 1) as dead_ratio
FROM pg_stat_user_tables
ORDER BY n_dead_tup DESC;
```

### Lock Monitoring

```sql
-- Check for blocking locks
SELECT
    blocked.pid AS blocked_pid,
    blocked.query AS blocked_query,
    blocking.pid AS blocking_pid,
    blocking.query AS blocking_query
FROM pg_stat_activity blocked
JOIN pg_locks blocked_locks ON blocked.pid = blocked_locks.pid
JOIN pg_locks blocking_locks ON blocked_locks.locktype = blocking_locks.locktype
    AND blocked_locks.relation = blocking_locks.relation
    AND blocked_locks.pid != blocking_locks.pid
JOIN pg_stat_activity blocking ON blocking_locks.pid = blocking.pid
WHERE NOT blocked_locks.granted;
```

## Maintenance

### Regular VACUUM

```sql
-- Reclaim space and update statistics
VACUUM ANALYZE tablename;

-- Full vacuum (locks table, use sparingly)
VACUUM FULL tablename;
```

### Reindex

```sql
-- Rebuild bloated indexes
REINDEX INDEX CONCURRENTLY indexname;

-- Rebuild all indexes on table
REINDEX TABLE CONCURRENTLY tablename;
```

### Statistics Update

```sql
-- Update planner statistics
ANALYZE tablename;

-- More detailed statistics for specific columns
ALTER TABLE tablename ALTER COLUMN columnname SET STATISTICS 1000;
ANALYZE tablename;
```
