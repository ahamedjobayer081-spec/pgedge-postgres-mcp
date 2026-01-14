/*-----------------------------------------------------------
 *
 * pgEdge Postgres MCP Server - MCP Protocol Implementation
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-----------------------------------------------------------
 */

# MCP Protocol Implementation

This document details the Model Context Protocol (MCP) implementation in the
pgEdge Postgres MCP Server.

## Overview

The MCP server implements JSON-RPC 2.0 protocol to provide AI models with
structured access to PostgreSQL databases. It exposes tools, resources, and
prompts that can be used by AI assistants.

## Protocol Basics

### JSON-RPC 2.0 Structure

**Request:**

```json
{
    "jsonrpc": "2.0",
    "id": "request-123",
    "method": "tools/call",
    "params": {
        "name": "query_database",
        "arguments": {
            "connection": "default",
            "query": "SELECT version()"
        }
    }
}
```

**Success Response:**

```json
{
    "jsonrpc": "2.0",
    "id": "request-123",
    "result": {
        "content": [
            {
                "type": "text",
                "text": "Query result: ..."
            }
        ]
    }
}
```

**Error Response:**

```json
{
    "jsonrpc": "2.0",
    "id": "request-123",
    "error": {
        "code": -32602,
        "message": "Invalid parameters",
        "data": "connection is required"
    }
}
```

## Handler Architecture

### Core Handler Structure

The MCP handler is located in `/internal/mcp/`:

```go
type Handler struct {
    tools     *tools.Registry
    resources *resources.Registry
    prompts   *prompts.Registry
    config    *config.Config
}

func NewHandler(
    tools *tools.Registry,
    resources *resources.Registry,
    prompts *prompts.Registry,
    cfg *config.Config,
) *Handler {
    return &Handler{
        tools:     tools,
        resources: resources,
        prompts:   prompts,
        config:    cfg,
    }
}
```

### Request Processing Flow

```go
func (h *Handler) HandleRequest(
    ctx context.Context,
    data []byte,
) (*Response, error) {
    // 1. Parse JSON-RPC request
    var req Request
    if err := json.Unmarshal(data, &req); err != nil {
        return NewErrorResponse(nil, ParseError, "Parse error", err.Error()), nil
    }

    // 2. Validate JSON-RPC version
    if req.JSONRPC != JSONRPCVersion {
        return NewErrorResponse(req.ID, InvalidRequest,
            "Invalid JSON-RPC version", nil), nil
    }

    // 3. Route to handler
    return h.routeRequest(ctx, req)
}
```

## Protocol Methods

### 1. initialize

Establishes the protocol session and exchanges capabilities.

**Request:**

```go
type InitializeParams struct {
    ProtocolVersion string                 `json:"protocolVersion"`
    Capabilities    map[string]interface{} `json:"capabilities"`
    ClientInfo      ClientInfo             `json:"clientInfo"`
}
```

**Response:**

```go
type InitializeResult struct {
    ProtocolVersion string                 `json:"protocolVersion"`
    Capabilities    map[string]interface{} `json:"capabilities"`
    ServerInfo      ServerInfo             `json:"serverInfo"`
}
```

### 2. tools/list

Lists available MCP tools.

**Response:**

```json
{
    "tools": [
        {
            "name": "query_database",
            "description": "Execute a SQL query on a connection",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "connection": {
                        "type": "string",
                        "description": "The connection name"
                    },
                    "query": {
                        "type": "string",
                        "description": "SQL query to execute"
                    }
                },
                "required": ["query"]
            }
        }
    ]
}
```

### 3. tools/call

Executes an MCP tool.

**Request:**

```json
{
    "name": "query_database",
    "arguments": {
        "connection": "default",
        "query": "SELECT version()"
    }
}
```

**Response:**

```json
{
    "content": [
        {
            "type": "text",
            "text": "Query executed successfully. Results: [...]"
        }
    ]
}
```

### 4. resources/list

Lists available resources.

### 5. resources/read

Retrieves resource data.

### 6. prompts/list

Lists available prompts.

### 7. prompts/get

Retrieves a prompt template.

## Tool Registry

Tools are registered in `/internal/tools/`:

```go
type Registry struct {
    tools map[string]Tool
    mu    sync.RWMutex
}

type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Execute(ctx context.Context, args map[string]interface{}) (
        []Content, error)
}

func (r *Registry) Register(tool Tool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.tools[tool.Name()] = tool
}
```

## Resource Registry

Resources are registered in `/internal/resources/`:

```go
type Registry struct {
    resources map[string]Resource
    mu        sync.RWMutex
}

type Resource interface {
    URI() string
    Name() string
    Description() string
    MimeType() string
    Read(ctx context.Context) ([]byte, error)
}
```

## Error Handling

### Error Codes

Following JSON-RPC 2.0 specification:

```go
const (
    ParseError     = -32700 // Invalid JSON
    InvalidRequest = -32600 // Invalid Request object
    MethodNotFound = -32601 // Method doesn't exist
    InvalidParams  = -32602 // Invalid parameters
    InternalError  = -32603 // Internal error
)
```

## Transport Modes

### HTTP Mode

The server can run as an HTTP server:

```go
http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read request", http.StatusBadRequest)
        return
    }

    resp, err := handler.HandleRequest(r.Context(), body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
})
```

### Stdio Mode

The server can also communicate via stdio for direct integration:

```go
func RunStdio(handler *Handler) error {
    reader := bufio.NewReader(os.Stdin)

    for {
        line, err := reader.ReadBytes('\n')
        if err != nil {
            if err == io.EOF {
                return nil
            }
            return err
        }

        resp, err := handler.HandleRequest(context.Background(), line)
        if err != nil {
            continue
        }

        json.NewEncoder(os.Stdout).Encode(resp)
    }
}
```

## Best Practices

1. **Always Validate Parameters:** Check required fields and types
2. **Use Transactions:** For operations that modify multiple tables
3. **Return Structured Errors:** Include context in error messages
4. **Handle Context Cancellation:** Respect request cancellation
5. **Release Resources:** Always release database connections
6. **Sanitize Output:** Don't leak sensitive data in error messages
