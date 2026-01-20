# PostgreSQL Security Best Practices

This document provides security guidance for PostgreSQL databases accessed
through the pgEdge Postgres MCP Server.

## Connection Security

### SSL/TLS Configuration

Always use encrypted connections in production:

```
postgresql://user:pass@host:5432/db?sslmode=verify-full
```

SSL modes (in order of security):

1. `verify-full`: Verify certificate and hostname (recommended)
2. `verify-ca`: Verify certificate against CA
3. `require`: Encrypted but no verification
4. `prefer`: Use SSL if available
5. `disable`: No encryption (never in production)

### Server Configuration

```ini
# postgresql.conf
ssl = on
ssl_cert_file = 'server.crt'
ssl_key_file = 'server.key'
ssl_ca_file = 'ca.crt'

# Minimum TLS version
ssl_min_protocol_version = 'TLSv1.2'
```

### pg_hba.conf

Restrict connections by source:

```
# TYPE  DATABASE  USER  ADDRESS       METHOD
hostssl all       all   10.0.0.0/8    scram-sha-256
hostssl all       all   192.168.0.0/16 scram-sha-256
host    all       all   0.0.0.0/0     reject
```

## Authentication

### Password Storage

Use SCRAM-SHA-256 (PostgreSQL 10+):

```sql
-- Set password encryption method
SET password_encryption = 'scram-sha-256';

-- Create user with SCRAM password
CREATE USER appuser WITH PASSWORD 'secure_password';
```

### Connection Limits

```sql
-- Limit connections per user
ALTER USER appuser CONNECTION LIMIT 10;

-- Limit connections per database
ALTER DATABASE appdb CONNECTION LIMIT 50;
```

### Password Policies

```sql
-- Set password expiration
ALTER USER appuser VALID UNTIL '2025-12-31';

-- Force password change
ALTER USER appuser PASSWORD NULL;
```

## Authorization

### Principle of Least Privilege

Grant only necessary permissions:

```sql
-- Create read-only role
CREATE ROLE readonly;
GRANT CONNECT ON DATABASE appdb TO readonly;
GRANT USAGE ON SCHEMA public TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly;

-- Assign to user
GRANT readonly TO appuser;
```

### Schema Isolation

```sql
-- Create application schema
CREATE SCHEMA app;

-- Grant schema access
GRANT USAGE ON SCHEMA app TO appuser;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA app TO appuser;

-- Set default privileges for new tables
ALTER DEFAULT PRIVILEGES IN SCHEMA app
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO appuser;
```

### Row-Level Security

```sql
-- Enable RLS on table
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;

-- Create policy
CREATE POLICY user_orders ON orders
    FOR ALL
    USING (user_id = current_setting('app.user_id')::int);
```

## SQL Injection Prevention

### Parameterized Queries

Always use parameters, never string concatenation:

```go
// GOOD: Parameterized
rows, err := pool.Query(ctx,
    "SELECT * FROM users WHERE id = $1", userID)

// BAD: String concatenation
rows, err := pool.Query(ctx,
    "SELECT * FROM users WHERE id = " + userID)  // NEVER DO THIS
```

### Input Validation

Validate user input before use:

```go
// Validate table names against allowlist
allowedTables := map[string]bool{
    "users": true,
    "orders": true,
}
if !allowedTables[tableName] {
    return errors.New("invalid table name")
}
```

### Identifier Quoting

When dynamic identifiers are necessary:

```go
// Use pgx's Identifier type
identifier := pgx.Identifier{schemaName, tableName}
sql := fmt.Sprintf("SELECT * FROM %s", identifier.Sanitize())
```

## Audit Logging

### Enable Logging

```ini
# postgresql.conf
log_statement = 'all'  # or 'ddl', 'mod'
log_connections = on
log_disconnections = on
log_duration = on
log_line_prefix = '%t [%p]: user=%u,db=%d,app=%a '
```

### pgAudit Extension

For detailed audit logging:

```sql
-- Install extension
CREATE EXTENSION pgaudit;

-- Configure auditing
ALTER SYSTEM SET pgaudit.log = 'write, ddl';
ALTER SYSTEM SET pgaudit.log_catalog = off;
SELECT pg_reload_conf();
```

## Data Protection

### Encryption at Rest

Use filesystem or volume encryption for data at rest.

For column-level encryption:

```sql
-- Using pgcrypto
CREATE EXTENSION pgcrypto;

-- Encrypt sensitive data
INSERT INTO users (email_encrypted)
VALUES (pgp_sym_encrypt('user@example.com', 'encryption_key'));

-- Decrypt when needed
SELECT pgp_sym_decrypt(email_encrypted, 'encryption_key') as email
FROM users;
```

### Sensitive Data Handling

```sql
-- Mask sensitive data in logs
ALTER SYSTEM SET log_parameter_max_length = 0;

-- Restrict access to sensitive columns
REVOKE SELECT (ssn, credit_card) ON users FROM public;
GRANT SELECT (id, name, email) ON users TO appuser;
```

## Network Security

### Firewall Rules

Restrict database port access:

```bash
# Allow only application servers
iptables -A INPUT -p tcp --dport 5432 -s 10.0.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 5432 -j DROP
```

### Connection Pooling Security

When using PgBouncer or similar:

- Use `auth_type = scram-sha-256`
- Enable TLS between pooler and database
- Restrict pooler admin access

## Monitoring and Alerts

### Failed Authentication

```sql
-- Check for failed logins (in logs)
-- grep "FATAL.*authentication" /var/log/postgresql/postgresql.log

-- Monitor connection attempts
SELECT datname, usename, client_addr, state
FROM pg_stat_activity
WHERE state = 'active';
```

### Privilege Changes

```sql
-- Monitor role grants
SELECT grantor, grantee, privilege_type, table_schema, table_name
FROM information_schema.role_table_grants
WHERE grantee = 'appuser';
```

## Security Checklist

- [ ] SSL/TLS enabled with verify-full
- [ ] SCRAM-SHA-256 authentication
- [ ] Restrictive pg_hba.conf rules
- [ ] Least privilege roles configured
- [ ] Row-level security where appropriate
- [ ] Audit logging enabled
- [ ] Parameterized queries only
- [ ] Network access restricted
- [ ] Regular security audits
- [ ] Password rotation policy
