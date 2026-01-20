# Codebase Navigator Knowledge Base

This directory contains documentation to help navigate the pgEdge Postgres MCP
Server codebase efficiently.

## Purpose

This knowledge base provides:

- Project structure and directory organization
- Feature implementation locations
- Data flow between components
- Key files and their purposes
- Common search patterns

## Documents

### [project-structure.md](project-structure.md)

High-level overview of the project organization:

- Directory layout for cmd/ and internal/
- Source code organization patterns
- Test file locations
- Configuration file locations

### [feature-locations.md](feature-locations.md)

Where specific features are implemented:

- Authentication and authorization
- Database connection management
- MCP tools and resources
- UI components and pages

### [data-flow.md](data-flow.md)

How data moves through the system:

- Client to server API patterns
- MCP request/response flow
- Database query patterns

### [key-files.md](key-files.md)

Critical files and their purposes:

- Entry points (cmd/)
- Configuration files
- Schema definitions
- Core business logic locations (internal/)

## Quick Reference

### Project Directories

- `/cmd/` - Entry points (MCP server, CLI, KB builder)
- `/internal/` - Core packages (mcp, auth, database, tools, resources)
- `/web/` - React web application (JavaScript/JSX)
- `/tests/` - Integration tests
- `/docs/` - Documentation

### Common Search Patterns

**Find MCP tool implementations:**

```
/.claude/mcp-expert/tools-catalog.md
/internal/tools/
```

**Find React components:**

```
/web/src/components/
/web/src/pages/
```

**Find database operations:**

```
/internal/database/
```

**Find test files:**

```
/internal/**/*_test.go
/cmd/**/*_test.go
/web/tests/
/tests/integration/
```

## Document Updates

Update these documents when:

- New features are added
- File locations change
- New patterns are established
- Data flow changes significantly

Last Updated: 2026-01-09
