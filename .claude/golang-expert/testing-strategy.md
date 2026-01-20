/*-----------------------------------------------------------
 *
 * pgEdge Postgres MCP Server - Testing Strategy
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-----------------------------------------------------------
 */

# Testing Strategy

This document describes the testing approach, patterns, and best practices for
the pgEdge Postgres MCP Server.

## Testing Philosophy

The project follows these testing principles:

1. **Test Behavior, Not Implementation:** Focus on what the code does
2. **Isolate Units:** Test components independently using mocking
3. **Test Realistic Scenarios:** Integration tests use real databases
4. **Maintain High Coverage:** Aim for >80% code coverage
5. **Fast Feedback:** Unit tests run in milliseconds
6. **Reliable Tests:** No flaky tests, consistent results

## Test Types

### 1. Unit Tests

Test individual functions and methods in isolation.

**Location:** Same directory as source code (Go convention)

**Example:** `/internal/mcp/server_test.go`

**Pattern:**

```go
package mcp

import (
    "testing"
)

func TestHandlerCreation(t *testing.T) {
    handler := NewHandler(nil, nil, nil, nil)

    if handler == nil {
        t.Fatal("NewHandler returned nil")
    }
}

func TestHandleInitialize(t *testing.T) {
    handler := NewHandler(nil, nil, nil, nil)

    reqData := []byte(`{
        "jsonrpc": "2.0",
        "id": "test-1",
        "method": "initialize",
        "params": {}
    }`)

    resp, err := handler.HandleRequest(context.Background(), reqData)
    if err != nil {
        t.Fatalf("HandleRequest failed: %v", err)
    }

    if resp.Error != nil {
        t.Errorf("Expected no error, got: %v", resp.Error)
    }
}
```

### 2. Integration Tests

Test components working together with real dependencies.

**Location:** Same directory with `_integration_test.go` suffix or `/tests/`

**Example:** `/internal/chat/client_integration_test.go`

**Pattern:**

```go
//go:build integration

package chat_test

import (
    "context"
    "testing"

    "pgedge-postgres-mcp/internal/chat"
)

func TestClientIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    client, err := chat.NewClient(testConfig)
    if err != nil {
        t.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Test interaction
    response, err := client.SendMessage(context.Background(), "test")
    if err != nil {
        t.Fatalf("Failed to send message: %v", err)
    }

    if response == "" {
        t.Error("Expected non-empty response")
    }
}
```

### 3. Table-Driven Tests

Test multiple scenarios with the same logic.

**Pattern:**

```go
func TestBuildConnectionString(t *testing.T) {
    tests := []struct {
        name     string
        config   ConnectionConfig
        contains string
    }{
        {
            name: "basic connection",
            config: ConnectionConfig{
                Host:     "localhost",
                Port:     5432,
                Database: "testdb",
                Username: "user",
            },
            contains: "host=localhost",
        },
        {
            name: "with SSL",
            config: ConnectionConfig{
                Host:     "localhost",
                Port:     5432,
                Database: "testdb",
                Username: "user",
                SSLMode:  "require",
            },
            contains: "sslmode=require",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := BuildConnectionString(tt.config)
            if !strings.Contains(result, tt.contains) {
                t.Errorf("Result %q should contain %q", result, tt.contains)
            }
        })
    }
}
```

## Running Tests

### Run All Tests

```bash
# Run all tests
make test

# Run with verbose output
go test -v ./...

# Run specific package
go test ./internal/mcp/...
```

### Run Specific Test

```bash
go test -run TestHandleInitialize ./internal/mcp/
```

### Run with Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

### Run with Race Detector

```bash
go test -race ./...
```

## Test Utilities

### Test Helpers

```go
func setupTestHandler(t *testing.T) *Handler {
    t.Helper()

    handler := NewHandler(nil, nil, nil, nil)
    if handler == nil {
        t.Fatal("Failed to create handler")
    }

    return handler
}

func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
}

func assertEqual(t *testing.T, got, want interface{}) {
    t.Helper()
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

### Database Test Helpers

Located in `/internal/database/test_helpers.go`:

```go
func SetupTestDatabase(t *testing.T) *pgxpool.Pool {
    t.Helper()

    connStr := os.Getenv("TEST_DATABASE_URL")
    if connStr == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }

    pool, err := pgxpool.New(context.Background(), connStr)
    if err != nil {
        t.Fatalf("Failed to create pool: %v", err)
    }

    t.Cleanup(func() {
        pool.Close()
    })

    return pool
}
```

## Test Organization

### Project Structure

```
internal/
├── mcp/
│   ├── server.go
│   └── server_test.go            # Unit tests
├── auth/
│   ├── auth.go
│   └── auth_test.go              # Unit tests
├── database/
│   ├── connection.go
│   ├── connection_test.go        # Unit tests
│   └── test_helpers.go           # Test utilities
└── chat/
    ├── client.go
    ├── client_test.go            # Unit tests
    └── client_integration_test.go # Integration tests
```

### Test Naming Conventions

```go
func TestFunctionName(t *testing.T)              // Basic test
func TestFunctionName_SpecificCase(t *testing.T) // Specific scenario
func TestFunctionName_Error(t *testing.T)        // Error case
```

## Mocking

### Interface-Based Mocking

```go
type DatabaseProvider interface {
    Query(ctx context.Context, query string, args ...interface{}) (
        pgx.Rows, error)
}

type mockDatabase struct {
    rows  pgx.Rows
    err   error
}

func (m *mockDatabase) Query(
    ctx context.Context,
    query string,
    args ...interface{},
) (pgx.Rows, error) {
    return m.rows, m.err
}

func TestWithMockDatabase(t *testing.T) {
    mock := &mockDatabase{err: nil}
    handler := NewHandler(mock)

    // Test with mock...
}
```

## Coverage Targets

- **Overall:** >80%
- **Critical Paths:** >90% (authentication, database, MCP handlers)
- **Utility Functions:** >70%

## Test Best Practices

### 1. Use t.Helper()

```go
func createTestConfig(t *testing.T) *Config {
    t.Helper()  // Errors reported at caller's line
    // ...
}
```

### 2. Use t.Cleanup()

```go
func TestWithCleanup(t *testing.T) {
    pool := setupPool(t)
    t.Cleanup(func() {
        pool.Close()
    })

    // Pool automatically closed after test
}
```

### 3. Use Subtests

```go
func TestOperations(t *testing.T) {
    t.Run("Create", func(t *testing.T) { /* ... */ })
    t.Run("Update", func(t *testing.T) { /* ... */ })
    t.Run("Delete", func(t *testing.T) { /* ... */ })
}
```

### 4. Avoid Sleeps

```go
// Bad
time.Sleep(100 * time.Millisecond)

// Good - use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
```

### 5. Test Error Conditions

```go
func TestDivision(t *testing.T) {
    tests := []struct {
        name    string
        a, b    int
        wantErr bool
    }{
        {"valid", 10, 2, false},
        {"division by zero", 10, 0, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := Divide(tt.a, tt.b)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 6. Use testdata Directory

```
pkg/
├── handler.go
├── handler_test.go
└── testdata/
    ├── valid_request.json
    └── invalid_request.json
```

### 7. Parallel Tests

```go
func TestParallel(t *testing.T) {
    t.Parallel()  // Mark as parallel-safe
    // ...
}
```

## Linting

### golangci-lint Configuration

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - staticcheck
    - unused
    - gosec
    - gofmt
```

### Running Linters

```bash
make lint
# or
golangci-lint run ./...
```

## Common Mistakes to Avoid

### 1. Not Cleaning Up Resources

```go
// Good
func TestWithCleanup(t *testing.T) {
    file, err := os.Create("test.txt")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() {
        os.Remove("test.txt")
        file.Close()
    })
}
```

### 2. Ignoring Errors in Tests

```go
// Bad
result, _ := SomeFunction()

// Good
result, err := SomeFunction()
if err != nil {
    t.Fatalf("SomeFunction() failed: %v", err)
}
```

### 3. Global State

```go
// Bad - shared global state
var testDB *pgxpool.Pool

// Good - test-local state
func TestSomething(t *testing.T) {
    db := setupDB(t)
    t.Cleanup(func() { db.Close() })
}
```
