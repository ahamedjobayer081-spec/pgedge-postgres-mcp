/*-----------------------------------------------------------
 *
 * pgEdge Postgres MCP Server - Authentication and Authorization
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-----------------------------------------------------------
 */

# Authentication and Authorization Flow

This document describes the authentication and authorization mechanisms
implemented in the pgEdge Postgres MCP Server.

## Overview

The MCP server supports flexible authentication to control access to tools,
resources, and database connections. Authentication is implemented in
`/internal/auth/`.

## Authentication Methods

### 1. Bearer Token Authentication

The server can authenticate requests using bearer tokens in the HTTP
Authorization header:

```http
POST /mcp HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
Content-Type: application/json
```

### 2. User-Based Authentication

Users can be defined in the configuration file with associated permissions:

```yaml
auth:
  enabled: true
  users:
    - username: admin
      password_hash: "sha256:..."
      roles: [admin]
    - username: analyst
      password_hash: "sha256:..."
      roles: [readonly]
```

### 3. API Key Authentication

API keys can be configured for service-to-service authentication:

```yaml
auth:
  api_keys:
    - name: "service-integration"
      key_hash: "sha256:..."
      roles: [query]
```

## Token Handling

### Token Validation

```go
func (a *Authenticator) ValidateToken(token string) (*UserInfo, error) {
    if token == "" {
        return nil, fmt.Errorf("no token provided")
    }

    // Check against configured tokens/API keys
    for _, apiKey := range a.config.APIKeys {
        if a.verifyHash(token, apiKey.KeyHash) {
            return &UserInfo{
                Username: apiKey.Name,
                Roles:    apiKey.Roles,
            }, nil
        }
    }

    return nil, fmt.Errorf("invalid token")
}
```

### Password Hashing

Passwords are stored as secure hashes:

```go
func HashPassword(password string) string {
    hash := sha256.Sum256([]byte(password))
    return fmt.Sprintf("sha256:%x", hash)
}

func VerifyPassword(password, hash string) bool {
    expected := HashPassword(password)
    return subtle.ConstantTimeCompare([]byte(expected), []byte(hash)) == 1
}
```

## Authorization

### Role-Based Access Control

The system implements role-based access control (RBAC):

```go
type Role struct {
    Name        string   `yaml:"name"`
    Permissions []string `yaml:"permissions"`
}

type Permission string

const (
    PermissionQueryDatabase   Permission = "query:database"
    PermissionReadResources   Permission = "read:resources"
    PermissionManageConnections Permission = "manage:connections"
)
```

### Checking Permissions

```go
func (a *Authorizer) HasPermission(user *UserInfo, permission Permission) bool {
    for _, role := range user.Roles {
        if a.roleHasPermission(role, permission) {
            return true
        }
    }
    return false
}

func (a *Authorizer) roleHasPermission(roleName string, permission Permission) bool {
    role, exists := a.roles[roleName]
    if !exists {
        return false
    }

    for _, perm := range role.Permissions {
        if perm == string(permission) || perm == "*" {
            return true
        }
    }
    return false
}
```

## Connection Access Control

### Per-User Connections

Users can be restricted to specific database connections:

```go
func (a *Authorizer) CanAccessConnection(
    user *UserInfo,
    connectionName string,
) bool {
    // Admins can access all connections
    if a.HasPermission(user, PermissionManageConnections) {
        return true
    }

    // Check user's allowed connections
    for _, allowed := range user.AllowedConnections {
        if allowed == "*" || allowed == connectionName {
            return true
        }
    }

    return false
}
```

## HTTP Middleware

Authentication is implemented as HTTP middleware:

```go
func AuthMiddleware(auth *Authenticator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractBearerToken(r)

            userInfo, err := auth.ValidateToken(token)
            if err != nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), userInfoKey, userInfo)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func extractBearerToken(r *http.Request) string {
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return ""
}
```

## Configuration

### Auth Configuration Structure

```yaml
auth:
  enabled: true

  # Optional: Skip auth for these paths
  public_paths:
    - /health
    - /ready

  # User accounts
  users:
    - username: admin
      password_hash: "sha256:..."
      roles: [admin]
      allowed_connections: ["*"]

    - username: analyst
      password_hash: "sha256:..."
      roles: [readonly]
      allowed_connections: ["analytics-db"]

  # API keys for service integration
  api_keys:
    - name: monitoring-service
      key_hash: "sha256:..."
      roles: [query]

  # Role definitions
  roles:
    - name: admin
      permissions: ["*"]

    - name: readonly
      permissions:
        - "query:database"
        - "read:resources"

    - name: query
      permissions:
        - "query:database"
```

## Security Best Practices

1. **Always Hash Credentials:** Never store plaintext passwords
2. **Use HTTPS:** Protect tokens in transit
3. **Rotate Tokens:** Implement token rotation policies
4. **Least Privilege:** Grant minimum required permissions
5. **Audit Logging:** Log authentication events
6. **Rate Limiting:** Prevent brute force attacks
7. **Constant-Time Comparison:** Prevent timing attacks

## Rate Limiting

The auth package includes rate limiting to prevent abuse:

```go
type RateLimiter struct {
    requests map[string][]time.Time
    mu       sync.RWMutex
    limit    int
    window   time.Duration
}

func (rl *RateLimiter) Allow(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-rl.window)

    // Clean old requests
    requests := rl.requests[key]
    valid := make([]time.Time, 0)
    for _, t := range requests {
        if t.After(cutoff) {
            valid = append(valid, t)
        }
    }

    if len(valid) >= rl.limit {
        return false
    }

    rl.requests[key] = append(valid, now)
    return true
}
```
