# Data Flow

This document describes how data moves through the pgEdge Postgres MCP Server
system.

## System Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   Server    │────▶│  Database   │
│   (React)   │◀────│   (MCP)     │◀────│ (PostgreSQL)│
└─────────────┘     └─────────────┘     └─────────────┘
```

## Client to Server Communication

### MCP Protocol Flow

1. **Client initiates connection**

   - Client: `POST /mcp` with JSON-RPC request
   - Server: Validates authentication token
   - Server: Routes to appropriate handler

2. **Tool execution**

   ```
   Client                    Server                    Database
     │                         │                          │
     │── tools/call ──────────▶│                          │
     │   {tool: "query_execute"│                          │
     │    params: {sql: "..."}}│                          │
     │                         │── Execute query ────────▶│
     │                         │◀── Results ──────────────│
     │◀── Result ─────────────│                          │
   ```

3. **Resource access**

   ```
   Client                    Server                    Database
     │                         │                          │
     │── resources/read ──────▶│                          │
     │   {uri: "pg://conn/1"}  │                          │
     │                         │── Fetch metadata ───────▶│
     │                         │◀── Connection info ──────│
     │◀── Resource content ───│                          │
   ```

### Authentication Flow

```
Client                    Server                    Database
  │                         │                          │
  │── Initialize ──────────▶│                          │
  │   (with auth token)     │                          │
  │                         │── Validate token ───────▶│
  │                         │◀── User info ────────────│
  │                         │── Check privileges ─────▶│
  │                         │◀── Permissions ──────────│
  │◀── Session info ───────│                          │
```

## Server Internal Data Flow

### Request Processing

```
HTTP Request
     │
     ▼
┌─────────────┐
│ HTTP Server │
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│    Auth     │────▶│   Session   │
│  Middleware │     │   Store     │
└──────┬──────┘     └─────────────┘
       │
       ▼
┌─────────────┐
│ MCP Handler │
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│Tool/Resource│────▶│  Database   │
│   Handler   │◀────│    Pool     │
└─────────────┘     └─────────────┘
```

### Authorization Check Flow

```
Request arrives
      │
      ▼
┌──────────────────┐
│ Extract token    │
│ from header      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Validate token   │
│ (hash lookup)    │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Get user groups  │
│ (recursive)      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Check privilege  │
│ for operation    │
└────────┬─────────┘
         │
         ▼
    Authorized?
    /        \
   Yes        No
    │          │
    ▼          ▼
 Execute    Error
```

## Database Schema Relationships

### Core Tables Flow

```
user_accounts
      │
      ├──▶ user_tokens
      │
      ├──▶ user_sessions
      │
      └──▶ group_memberships ──▶ user_groups
                                      │
                                      ├──▶ group_mcp_privileges
                                      │
                                      └──▶ group_connection_privileges
                                                     │
                                                     ▼
                                               connections
```

## Client State Flow (Planned)

### React Component Hierarchy

```
App
 │
 ├── AuthProvider (context)
 │       │
 │       └── User state, tokens
 │
 ├── ConnectionProvider (context)
 │       │
 │       └── Active connections, selection
 │
 └── Pages
         │
         ├── Dashboard
         │       └── Fetches: metrics, status
         │
         ├── Connections
         │       └── Fetches: connection list, CRUD
         │
         └── Query
                 └── Fetches: query execution, results
```

### API Call Pattern

```typescript
// Typical data flow in React component
Component
    │
    ├── useEffect (mount)
    │       │
    │       └── Call API service
    │               │
    │               └── Fetch from server
    │                       │
    │                       └── Update state
    │
    └── Render with state
```

## Error Flow

### Server Error Handling

```
Error occurs
      │
      ▼
┌──────────────────┐
│ Wrap with context│
│ (fmt.Errorf)     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Log with details │
│ (internal)       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Return generic   │
│ error to client  │
└──────────────────┘
```

### Client Error Handling

```
API error received
        │
        ▼
┌──────────────────┐
│ Parse error      │
│ response         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Display user-    │
│ friendly message │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Log details      │
│ (if debug mode)  │
└──────────────────┘
```
