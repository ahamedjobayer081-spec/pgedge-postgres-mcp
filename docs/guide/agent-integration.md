# Configuring AI Agent Tool Selection

AI coding agents such as Claude Code and Cursor
sometimes prefer shell commands over MCP tools for
database operations. Training data for these agents
contains many examples of shell-based workflows
using `psql` and similar utilities. The
MCP server includes built-in guidance to steer agents
toward its tools, but project-level configuration
can reinforce this behavior.

## Server-Level Instructions

The MCP server automatically sends instructions to
compatible clients during initialization. These
instructions tell the agent to prefer the MCP
server tools over shell commands. The server sends
the instructions through the MCP protocol's
`instructions` field during the `initialize`
handshake.

No user configuration is required for this feature.
The server delivers tool-preference guidance to any
MCP-compatible client automatically.

## Project-Level Configuration

Users can add instructions to project configuration
files to reinforce tool selection. Each AI coding
agent reads a specific configuration file from the
project root directory.

### Claude Code

Claude Code reads project instructions from a
`CLAUDE.md` file in the project root directory.

In the following example, the `CLAUDE.md` file
includes a database access constraint:

```markdown
## Database Access

All PostgreSQL operations in this project use the
pgEdge MCP tools. Do not use psql or direct shell
database commands for querying or schema inspection.
```

Add this block to the `CLAUDE.md` file in your
project root. Framing the instruction as a project
constraint rather than a preference makes the
instruction more effective. Agents treat constraints
as rules rather than suggestions.

### Cursor

Cursor reads project instructions from a
`.cursorrules` file in the project root directory.

In the following example, the `.cursorrules` file
includes a tool selection instruction:

```
For all PostgreSQL database operations, use the
pgEdge MCP server tools instead of psql or shell
commands. The MCP tools handle connection
management, authentication, and access control
automatically.
```

Add this block to the `.cursorrules` file in your
project root. Cursor applies these rules to all
conversations within the project.

## Tips for Effective Tool Selection

The following tips help maintain consistent tool
selection across agent sessions:

- Keep agent instructions concise and frame them
  as project constraints.
- The MCP server tool descriptions include
  "use this instead of..." guidance that agents
  read when deciding which tool to call.
- If an agent still defaults to shell commands,
  explicitly ask the agent to use the MCP tools
  for the current task.
- Long sessions may cause instruction drift;
  reinforce the tool preference if the agent
  reverts to shell commands.
