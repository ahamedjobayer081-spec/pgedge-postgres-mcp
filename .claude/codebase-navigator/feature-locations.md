# Feature Locations

This document maps features to their implementation locations in the codebase.

## Authentication & Authorization

### User Authentication

| Feature | Location | Description |
|---------|----------|-------------|
| Password hashing | `/internal/auth/` | User password storage and verification |
| Token generation | `/internal/auth/tokens.go` | Service and user token creation |
| Token validation | `/internal/auth/tokens.go` | Token verification and lookup |
| Session management | `/internal/auth/sessions.go` | User session lifecycle |

### Role-Based Access Control (RBAC)

| Feature | Location | Description |
|---------|----------|-------------|
| Group management | `/internal/auth/rbac.go` | User groups and memberships |
| Privilege checking | `/internal/auth/rbac.go` | Authorization verification |
| MCP privileges | `/internal/auth/` | Tool and resource access control |
| Connection privileges | `/internal/auth/` | Database connection access |

### Database Schema

| Table | Purpose |
|-------|---------|
| `user_accounts` | User credentials and metadata |
| `user_tokens` | User API tokens |
| `service_tokens` | Service API tokens |
| `user_sessions` | Active user sessions |
| `user_groups` | Authorization groups |
| `group_memberships` | User-to-group mappings |
| `group_mcp_privileges` | MCP tool/resource permissions |
| `group_connection_privileges` | Database connection permissions |

## Database Connection Management

### Server-Side

| Feature | Location | Description |
|---------|----------|-------------|
| Connection pool | `/internal/database/pool.go` | pgx connection pooling |
| Connection CRUD | `/internal/database/` | Create, read, update, delete connections |
| Credential encryption | `/internal/database/` | Password encryption at rest |

### MCP Tools

| Tool | Location | Purpose |
|------|----------|---------|
| `connection_create` | `/internal/tools/` | Create new connection |
| `connection_list` | `/internal/tools/` | List available connections |
| `connection_update` | `/internal/tools/` | Modify connection settings |
| `connection_delete` | `/internal/tools/` | Remove connection |
| `connection_test` | `/internal/tools/` | Test connection validity |

## MCP Protocol Implementation

### Core Protocol

| Feature | Location | Description |
|---------|----------|-------------|
| JSON-RPC handler | `/internal/mcp/server.go` | Request routing and response |
| Protocol methods | `/internal/mcp/` | initialize, ping, etc. |
| Error handling | `/internal/mcp/` | MCP error responses |

### Tools

| Category | Location | Examples |
|----------|----------|----------|
| Connection tools | `/internal/tools/` | CRUD operations |
| Query tools | `/internal/tools/` | SQL execution |
| User tools | `/internal/tools/` | User management |
| Privilege tools | `/internal/tools/` | RBAC management |

### Resources

| Resource | Location | Purpose |
|----------|----------|---------|
| Connection info | `/internal/resources/` | Connection metadata |
| Schema info | `/internal/resources/` | Database schema details |

## Web Client Features (Planned)

### Pages

| Page | Location | Purpose |
|------|----------|---------|
| Dashboard | `/web/src/pages/` | Main overview |
| Connections | `/web/src/pages/` | Connection management |
| Query | `/web/src/pages/` | SQL query interface |
| Settings | `/web/src/pages/` | User preferences |

### Components

| Component | Location | Purpose |
|-----------|----------|---------|
| Connection list | `/web/src/components/` | Display connections |
| Query editor | `/web/src/components/` | SQL input |
| Results table | `/web/src/components/` | Query results display |
| Navigation | `/web/src/components/` | App navigation |

### State Management

| Store | Location | Purpose |
|-------|----------|---------|
| Auth state | `/web/src/contexts/` | User authentication |
| Connection state | `/web/src/contexts/` | Active connections |
| Query state | `/web/src/contexts/` | Query history and results |

## Configuration

### Server Configuration

| Setting | Location | Description |
|---------|----------|-------------|
| Database URL | Environment/config | Server database connection |
| Listen address | Environment/config | HTTP server bind address |
| Auth settings | Environment/config | Token expiration, etc. |

### Client Configuration (Planned)

| Setting | Location | Description |
|---------|----------|-------------|
| API URL | Environment/config | Server API endpoint |
| Theme | `/web/src/theme/` | MUI theme settings |
