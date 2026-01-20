# Configuring the MCP Server for Multiple Databases

The MCP server supports configuring multiple PostgreSQL databases,
allowing users to switch between different database connections at runtime.
This is particularly useful for environments with separate development,
staging, and production databases, or when providing access to multiple
projects.

## Configuring Multiple Databases

Each database must have a unique name that users reference when switching
connections:

```yaml
databases:
  - name: "production"
    host: "prod-db.example.com"
    port: 5432
    database: "myapp"
    user: "readonly_user"
    sslmode: "require"
    available_to_users: []  # All users can access

  - name: "staging"
    host: "staging-db.example.com"
    port: 5432
    database: "myapp_staging"
    user: "developer"
    sslmode: "prefer"
    available_to_users:
      - "alice"
      - "bob"
      - "qa_team"

  - name: "development"
    host: "localhost"
    port: 5432
    database: "myapp_dev"
    user: "developer"
    sslmode: "disable"
    available_to_users:
      - "alice"
      - "bob"
```

### Access Control

The `available_to_users` field controls which session users can access each
database:

- **Empty list (`[]`)**: All authenticated users can access the database
- **User list**: Only the specified usernames can access the database
- **API tokens**: Bound to a specific database via the token's `database` field
  (see [Authentication Guide](authentication.md))

**Access control is enforced in HTTP mode only.** In STDIO mode or when
authentication is disabled (`--no-auth`), all databases are accessible to
everyone.

### Default Database Selection

When a user connects, the system automatically selects a default database
using this priority:

1. **Saved preference**: If the user previously selected a database and it's
   still accessible, that database is used
2. **First accessible database**: Otherwise, the first database in the
   configuration list that the user has access to is selected
3. **No database**: If no databases are accessible, database operations will
   fail with an appropriate error message

**Example scenarios:**

| User | Accessible Databases | Default Selection |
|------|---------------------|-------------------|
| alice | production, staging, development | production (first) |
| bob | production, staging, development | production (first) |
| qa_team | production, staging | production (first) |
| guest | production | production (only option) |
| unknown | (none) | Error: no accessible databases |

### Runtime Database Switching

Users can switch between accessible databases at runtime using the client
interfaces:

**CLI Client:**

```
/list databases        # Show available databases
/show database         # Show current database
/set database staging  # Switch to staging database
```

**Web UI:**

Click the database icon in the status banner to open the database selector.
Select a database from the list to switch connections.

**Note:** Database switching is disabled while an LLM query is being
processed to prevent data consistency issues.

### LLM Database Switching

You can optionally allow the LLM to list and switch databases using MCP
tools. This feature is disabled by default for security reasons.

To enable LLM database switching, add the following to your configuration:

```yaml
builtins:
  tools:
    llm_connection_selection: true
```

When enabled, the LLM has access to two additional tools:

- `list_database_connections`: Lists databases available for switching
- `select_database_connection`: Switches to a specified database

#### Excluding Databases from LLM Switching

You can prevent specific databases from being visible to LLM switching
tools using the `allow_llm_switching` option:

```yaml
databases:
  - name: "production"
    host: "prod-db.example.com"
    database: "myapp"
    allow_llm_switching: false  # Hidden from LLM

  - name: "staging"
    host: "staging-db.example.com"
    database: "myapp_staging"
    # allow_llm_switching defaults to true
```

When `allow_llm_switching: false` is set:

- The database does not appear in `list_database_connections` results
- Attempts to switch to it via `select_database_connection` are denied
- Manual switching via CLI commands or web UI is unaffected
- API token bindings and `available_to_users` restrictions still apply

This allows administrators to grant LLM access to development and staging
databases while keeping production databases accessible only through
manual user selection.

### Database Selection Persistence

When a user selects a database:

- The selection is saved to the user's session preferences
- On subsequent connections, the saved preference is restored (if still
  accessible)
- If the preferred database is no longer accessible (e.g., removed from
  configuration or user permissions changed), the system falls back to the
  first accessible database

