# Project Structure

This document describes the directory organization of the pgEdge Postgres MCP
Server.

## Top-Level Layout

```
pgedge-nla/
├── .claude/              # Claude Code agent definitions and knowledge bases
├── .github/              # GitHub Actions workflows
├── cmd/                  # Entry points
│   ├── pgedge-pg-mcp-svr/   # MCP server
│   ├── pgedge-pg-mcp-cli/   # CLI client
│   └── kb-builder/          # Knowledge base builder
├── internal/             # Core packages (private)
├── web/                  # React web application (JavaScript/JSX)
├── docs/                 # Project documentation
├── tests/                # Integration tests
├── CLAUDE.md             # Claude Code instructions
├── Makefile              # Top-level build commands
└── README.md             # Project overview
```

## Entry Points (`/cmd`)

Each binary has its own subdirectory under cmd/.

```
cmd/
├── pgedge-pg-mcp-svr/
│   └── main.go           # MCP server entry point
├── pgedge-pg-mcp-cli/
│   └── main.go           # CLI client entry point
└── kb-builder/
    └── main.go           # Knowledge base builder entry point
```

## Internal Packages (`/internal`)

Core implementation following Go standard layout.

```
internal/
├── api/                  # HTTP API handlers
├── auth/                 # Authentication and authorization
│   ├── tokens.go         # Token management
│   ├── rbac.go           # Role-based access control
│   └── sessions.go       # Session management
├── database/             # Database operations
│   ├── schema.go         # Migration definitions
│   └── pool.go           # Connection pooling
├── mcp/                  # MCP protocol implementation
│   ├── server.go         # MCP server and handlers
│   ├── http_server.go    # HTTP/SSE server
│   └── types.go          # Protocol types
├── tools/                # MCP tool implementations
├── resources/            # MCP resource implementations
├── prompts/              # MCP prompt implementations
├── config/               # Configuration loading
└── *_test.go             # Unit tests (co-located)
```

## Web Client (`/web`)

The React web application for user interaction (JavaScript/JSX).

```
web/
├── src/
│   ├── main.jsx          # Entry point
│   ├── App.jsx           # Root component
│   ├── components/       # React components
│   │   ├── Header.jsx
│   │   ├── ChatInterface.jsx
│   │   ├── MessageList.jsx
│   │   └── __tests__/    # Component tests
│   ├── contexts/         # React contexts for state
│   │   ├── AuthContext.jsx
│   │   └── DatabaseContext.jsx
│   ├── theme/            # MUI theme configuration
│   │   └── pgedgeTheme.js
│   └── utils/            # Utility functions
├── public/               # Static assets
├── package.json          # Dependencies
├── vite.config.js        # Vite build configuration
└── vitest.config.js      # Test configuration
```

## Tests Structure (`/tests`)

Integration tests spanning multiple components.

```
tests/
├── integration/          # Integration test files
├── testutil/             # Test utilities
│   ├── database.go       # Database test helpers
│   ├── services.go       # Service management helpers
│   ├── config.go         # Configuration helpers
│   └── common.go         # Common utilities
├── logs/                 # Test execution logs
├── Makefile              # Test execution commands
└── README.md             # Test documentation
```

## Documentation Structure (`/docs`)

Project documentation.

```
docs/
├── index.md              # Documentation entry point
├── quickstart/           # Getting started guides
├── configuration/        # Configuration reference
├── api/                  # API documentation
└── LICENSE.md            # Project license
```

## Configuration Files

Key configuration files and their locations:

| File | Location | Purpose |
|------|----------|---------|
| `Makefile` | Root | Build, test, lint commands |
| `go.mod` | Root | Go module dependencies |
| `package.json` | `/web` | Node.js dependencies |
| `vite.config.js` | `/web` | Vite build configuration |
| `.golangci.yml` | Root | Linter configuration |

## Source Code Conventions

The project follows these conventions:

- Entry points in `/cmd/<binary>/`
- Private packages in `/internal/`
- Tests co-located with source (`*_test.go`)
- Four-space indentation
- Documentation in `/docs/`
