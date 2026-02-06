# Client Examples

This guide provides complete, runnable client examples in
Python and JavaScript for the pgEdge Postgres MCP Server.

## Overview

The MCP server exposes a JSON-RPC 2.0 API over HTTP. All
tool calls use `POST /mcp/v1` with `method: "tools/call"`.
The server requires a Bearer token in the `Authorization`
header for authenticated requests.

Each request follows the JSON-RPC 2.0 envelope format.
The following fields are required in every request:

- The `jsonrpc` field must contain the string `"2.0"`.
- The `id` field must contain a unique request identifier.
- The `method` field specifies the MCP method to invoke.
- The `params` field contains the tool name and arguments.

In the following example, a `tools/call` request executes
the `query_database` tool:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
        "name": "query_database",
        "arguments": {
            "query": "SELECT version()"
        }
    }
}
```

The server returns results in the MCP content format. In
the following example, the response contains a text result:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "PostgreSQL 17.0 on x86_64"
            }
        ]
    }
}
```

## Python Client

This section provides a complete Python client class and
usage examples for the MCP server.

### Prerequisites

The Python client requires the `requests` library. In the
following command, `pip` installs the dependency:

```bash
pip install requests
```

### Complete Client Class

The following class encapsulates authentication, tool
invocation, and token lifecycle management. The class
handles JSON-RPC formatting and Bearer token headers
automatically.

```python
import requests
import json
from datetime import datetime, timezone


class PgEdgeMCPClient:
    """Client for the pgEdge Postgres MCP Server."""

    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url
        self.session_token = None
        self.token_expiry = None
        self._request_id = 0

    def _next_id(self):
        self._request_id += 1
        return self._request_id

    def _headers(self):
        headers = {
            "Content-Type": "application/json"
        }
        if self.session_token:
            headers["Authorization"] = (
                f"Bearer {self.session_token}"
            )
        return headers

    def _is_token_expired(self):
        if not self.token_expiry:
            return True
        expiry = datetime.fromisoformat(
            self.token_expiry.replace(
                "Z", "+00:00"
            )
        )
        return datetime.now(timezone.utc) >= expiry

    def authenticate(self, username, password):
        """Authenticate and store session token."""
        response = requests.post(
            f"{self.base_url}/mcp/v1",
            json={
                "jsonrpc": "2.0",
                "id": self._next_id(),
                "method": "tools/call",
                "params": {
                    "name": "authenticate_user",
                    "arguments": {
                        "username": username,
                        "password": password
                    }
                }
            }
        )
        response.raise_for_status()
        result = response.json()

        if "error" in result:
            raise Exception(
                f"Auth failed: {result['error']}"
            )

        auth_data = json.loads(
            result["result"]["content"][0]["text"]
        )
        self.session_token = (
            auth_data["session_token"]
        )
        self.token_expiry = (
            auth_data["expires_at"]
        )
        return auth_data

    def _ensure_authenticated(
        self, username=None, password=None
    ):
        """Re-authenticate if token has expired."""
        if self._is_token_expired():
            if username and password:
                self.authenticate(
                    username, password
                )
            else:
                raise Exception("Token expired")

    def call_tool(self, tool_name, arguments=None):
        """Call an MCP tool and return the result."""
        if arguments is None:
            arguments = {}

        response = requests.post(
            f"{self.base_url}/mcp/v1",
            headers=self._headers(),
            json={
                "jsonrpc": "2.0",
                "id": self._next_id(),
                "method": "tools/call",
                "params": {
                    "name": tool_name,
                    "arguments": arguments
                }
            }
        )

        if response.status_code == 401:
            raise Exception(
                "Unauthorized: token expired"
            )

        response.raise_for_status()
        result = response.json()

        if "error" in result:
            raise Exception(
                f"Tool error: {result['error']}"
            )

        return (
            result["result"]["content"][0]["text"]
        )

    def query_database(self, query):
        """Execute a SQL query."""
        return self.call_tool(
            "query_database", {"query": query}
        )

    def get_schema(
        self, schema_name=None, table_name=None
    ):
        """Retrieve database schema information."""
        args = {}
        if schema_name:
            args["schema_name"] = schema_name
        if table_name:
            args["table_name"] = table_name
        return self.call_tool(
            "get_schema_info", args
        )

    def list_databases(self):
        """List accessible databases."""
        response = requests.get(
            f"{self.base_url}/api/databases",
            headers=self._headers()
        )
        response.raise_for_status()
        return response.json()

    def select_database(self, name):
        """Switch to a different database."""
        response = requests.post(
            f"{self.base_url}/api/databases/select",
            headers=self._headers(),
            json={"name": name}
        )
        response.raise_for_status()
        return response.json()

    def search_knowledgebase(
        self, query, project_names=None, top_n=5
    ):
        """Search the documentation knowledgebase."""
        args = {"query": query, "top_n": top_n}
        if project_names:
            args["project_names"] = project_names
        return self.call_tool(
            "search_knowledgebase", args
        )
```

### Authentication

The `authenticate` method sends credentials and stores the
returned session token. The token remains valid for 24
hours from the time of authentication.

In the following example, the client authenticates with a
username and password:

```python
client = PgEdgeMCPClient("http://localhost:8080")
auth = client.authenticate(
    "alice", "SecurePassword123!"
)
print(f"Token expires: {client.token_expiry}")
```

The server returns a JSON object containing the session
token and expiry timestamp:

```json
{
    "success": true,
    "session_token": "AQz9XfK...",
    "expires_at": "2025-11-15T09:30:00Z",
    "message": "Authentication successful"
}
```

### Schema Retrieval

The `get_schema_info` tool retrieves database structure
information. The tool supports compact and detailed modes.

In the following example, the client retrieves a compact
overview of all schemas:

```python
schema = client.call_tool(
    "get_schema_info", {"compact": True}
)
print(schema)
```

In the following example, the client retrieves detailed
column information for a specific table:

```python
users = client.get_schema("public", "users")
print(users)
```

### Parsing TSV Results

The `query_database` and `get_schema_info` tools return
results in TSV (tab-separated values) format. The following
helper function parses TSV text into a list of
dictionaries.

In the following example, the `parse_tsv` function converts
raw TSV output into structured Python objects:

```python
def parse_tsv(tsv_text):
    """Parse TSV results into a list of dicts."""
    lines = tsv_text.strip().split("\n")
    data_lines = [
        line for line in lines
        if "\t" in line
    ]
    if not data_lines:
        return []
    headers = data_lines[0].split("\t")
    rows = []
    for line in data_lines[1:]:
        values = line.split("\t")
        rows.append(
            dict(zip(headers, values))
        )
    return rows


result = client.query_database(
    "SELECT id, name FROM users LIMIT 5"
)
rows = parse_tsv(result)
for row in rows:
    print(f"User: {row.get('name', 'N/A')}")
```

### Query Execution with Error Handling

The client should handle HTTP errors and re-authenticate
when the session token expires. The server returns a `401`
status code when the token is invalid or expired.

In the following example, the client catches a `401` error
and re-authenticates before retrying the query:

```python
try:
    result = client.query_database(
        "SELECT * FROM users LIMIT 10"
    )
    print(result)
except requests.exceptions.HTTPError as e:
    if e.response.status_code == 401:
        print("Session expired; re-authenticating")
        client.authenticate(
            "alice", "SecurePassword123!"
        )
        result = client.query_database(
            "SELECT * FROM users LIMIT 10"
        )
    else:
        raise
except Exception as e:
    print(f"Query failed: {e}")
```

### Database Switching

The REST API provides endpoints for listing and selecting
databases. The client can switch databases without creating
a new session.

In the following example, the client lists all accessible
databases and switches to a different database:

```python
databases = client.list_databases()
print(f"Current: {databases['current']}")
for db in databases["databases"]:
    print(f"  - {db['name']} ({db['host']})")

result = client.select_database("staging")
print(f"Switched to: {result['current']}")
```

The `list_databases` endpoint returns a JSON object with
the following structure:

```json
{
    "databases": [
        {
            "name": "production",
            "host": "localhost",
            "port": 5432,
            "database": "myapp",
            "user": "appuser",
            "sslmode": "require"
        },
        {
            "name": "staging",
            "host": "staging.example.com",
            "port": 5432,
            "database": "myapp_staging",
            "user": "appuser",
            "sslmode": "require"
        }
    ],
    "current": "production"
}
```

### Knowledgebase Search

The `search_knowledgebase` tool searches pre-built
documentation for relevant information. The tool supports
filtering by project name and version.

In the following example, the client searches for
PostgreSQL documentation about window functions:

```python
results = client.search_knowledgebase(
    "window functions",
    project_names=["PostgreSQL"]
)
print(results)
```

### Token Lifecycle with Auto-Retry

Long-running applications should handle token expiry
gracefully. The following helper function retries queries
with automatic re-authentication on failure.

In the following example, the `query_with_retry` function
attempts a query and re-authenticates if the token expires:

```python
import time


def query_with_retry(
    client, query, username, password,
    max_retries=3
):
    """Execute a query with automatic retry."""
    for attempt in range(max_retries):
        try:
            return client.query_database(query)
        except Exception as e:
            is_auth_error = (
                "401" in str(e)
                or "expired" in str(e)
            )
            if is_auth_error:
                client.authenticate(
                    username, password
                )
                continue
            if attempt < max_retries - 1:
                time.sleep(2 ** attempt)
                continue
            raise


result = query_with_retry(
    client,
    "SELECT COUNT(*) FROM orders",
    "alice",
    "SecurePassword123!"
)
print(result)
```

## JavaScript Client

This section provides a complete JavaScript client class
that uses the Fetch API with `async`/`await` syntax.

### Complete Client Class

The following class handles authentication, tool calls, and
database operations. The class works in both Node.js and
browser environments.

```javascript
class PgEdgeMCPClient {
    constructor(baseUrl = "http://localhost:8080") {
        this.baseUrl = baseUrl;
        this.sessionToken = null;
        this.tokenExpiry = null;
        this.requestId = 0;
    }

    _nextId() {
        return ++this.requestId;
    }

    _headers() {
        const headers = {
            "Content-Type": "application/json"
        };
        if (this.sessionToken) {
            headers["Authorization"] =
                `Bearer ${this.sessionToken}`;
        }
        return headers;
    }

    isTokenExpired() {
        if (!this.tokenExpiry) return true;
        const expiry = new Date(this.tokenExpiry);
        return new Date() >= expiry;
    }

    async authenticate(username, password) {
        const response = await fetch(
            `${this.baseUrl}/mcp/v1`,
            {
                method: "POST",
                headers: {
                    "Content-Type":
                        "application/json"
                },
                body: JSON.stringify({
                    jsonrpc: "2.0",
                    id: this._nextId(),
                    method: "tools/call",
                    params: {
                        name:
                            "authenticate_user",
                        arguments: {
                            username,
                            password
                        }
                    }
                })
            }
        );

        if (!response.ok) {
            throw new Error(
                `Auth failed: ${response.status}`
            );
        }

        const result = await response.json();
        if (result.error) {
            throw new Error(
                `Auth error: `
                + result.error.message
            );
        }

        const authData = JSON.parse(
            result.result.content[0].text
        );
        this.sessionToken =
            authData.session_token;
        this.tokenExpiry =
            authData.expires_at;
        return authData;
    }

    async callTool(toolName, args = {}) {
        const response = await fetch(
            `${this.baseUrl}/mcp/v1`,
            {
                method: "POST",
                headers: this._headers(),
                body: JSON.stringify({
                    jsonrpc: "2.0",
                    id: this._nextId(),
                    method: "tools/call",
                    params: {
                        name: toolName,
                        arguments: args
                    }
                })
            }
        );

        if (response.status === 401) {
            throw new Error("Token expired");
        }

        if (!response.ok) {
            throw new Error(
                `Request failed: `
                + response.status
            );
        }

        const result = await response.json();
        if (result.error) {
            throw new Error(
                result.error.message
            );
        }

        return result.result.content[0].text;
    }

    async queryDatabase(query) {
        return this.callTool(
            "query_database", { query }
        );
    }

    async getSchema(schemaName, tableName) {
        const args = {};
        if (schemaName) {
            args.schema_name = schemaName;
        }
        if (tableName) {
            args.table_name = tableName;
        }
        return this.callTool(
            "get_schema_info", args
        );
    }

    async listDatabases() {
        const response = await fetch(
            `${this.baseUrl}/api/databases`,
            { headers: this._headers() }
        );
        if (!response.ok) {
            throw new Error(
                `Request failed: `
                + response.status
            );
        }
        return response.json();
    }

    async selectDatabase(name) {
        const response = await fetch(
            `${this.baseUrl}/api/databases/select`,
            {
                method: "POST",
                headers: this._headers(),
                body: JSON.stringify({ name })
            }
        );
        if (!response.ok) {
            throw new Error(
                `Request failed: `
                + response.status
            );
        }
        return response.json();
    }

    async searchKnowledgebase(
        query, projectNames, topN = 5
    ) {
        const args = {
            query: query,
            top_n: topN
        };
        if (projectNames) {
            args.project_names = projectNames;
        }
        return this.callTool(
            "search_knowledgebase", args
        );
    }
}
```

### JavaScript Usage Examples

The following examples demonstrate common operations with
the JavaScript client. Each example uses `async`/`await`
syntax inside an async function.

In the following example, the client authenticates and
executes a database query:

```javascript
async function main() {
    const client = new PgEdgeMCPClient(
        "http://localhost:8080"
    );

    // Authenticate with credentials
    const auth = await client.authenticate(
        "alice", "SecurePassword123!"
    );
    console.log(
        `Token expires: ${client.tokenExpiry}`
    );

    // Execute a SQL query
    const result = await client.queryDatabase(
        "SELECT id, name FROM users LIMIT 5"
    );
    console.log(result);

    // Retrieve schema information
    const schema = await client.getSchema(
        "public", "users"
    );
    console.log(schema);

    // List available databases
    const databases =
        await client.listDatabases();
    console.log(
        `Current: ${databases.current}`
    );

    // Switch to a different database
    const switched =
        await client.selectDatabase("staging");
    console.log(
        `Switched to: ${switched.current}`
    );

    // Search the knowledgebase
    const docs =
        await client.searchKnowledgebase(
            "window functions",
            ["PostgreSQL"]
        );
    console.log(docs);
}

main().catch(console.error);
```

### Parsing TSV Results in JavaScript

The following function parses TSV output into an array of
objects. The function skips non-TSV header lines that the
server may include before the tabular data.

In the following example, the `parseTsv` function converts
raw TSV text into structured JavaScript objects:

```javascript
function parseTsv(tsvText) {
    const lines = tsvText.trim().split("\n");
    const dataLines = lines.filter(
        line => line.includes("\t")
    );
    if (dataLines.length === 0) return [];

    const headers = dataLines[0].split("\t");
    return dataLines.slice(1).map(line => {
        const values = line.split("\t");
        const row = {};
        headers.forEach((header, index) => {
            row[header] = values[index] || "";
        });
        return row;
    });
}

// Usage with the client
async function queryAndParse(client) {
    const result = await client.queryDatabase(
        "SELECT id, name FROM users LIMIT 5"
    );
    const rows = parseTsv(result);
    for (const row of rows) {
        console.log(`User: ${row.name}`);
    }
}
```

### Error Handling with Auto-Retry

The following function wraps tool calls with automatic
retry logic. The function re-authenticates when the server
returns a `401` status code.

In the following example, the `queryWithRetry` function
retries a failed query with exponential backoff:

```javascript
async function queryWithRetry(
    client, query, username, password,
    maxRetries = 3
) {
    for (let attempt = 0;
         attempt < maxRetries;
         attempt++) {
        try {
            return await client.queryDatabase(
                query
            );
        } catch (error) {
            const msg = error.message;
            const isAuthError =
                msg.includes("401")
                || msg.includes("expired");

            if (isAuthError) {
                await client.authenticate(
                    username, password
                );
                continue;
            }

            if (attempt < maxRetries - 1) {
                const delay = 2 ** attempt;
                await new Promise(
                    r => setTimeout(r,
                        delay * 1000)
                );
                continue;
            }
            throw error;
        }
    }
}
```

## Using API Tokens

Pre-configured API tokens allow access without interactive
login. An administrator creates API tokens in the server
configuration file.

### Python with API Token

In the following example, the Python client uses a
pre-configured API token instead of user authentication:

```python
client = PgEdgeMCPClient("http://localhost:8080")
client.session_token = "your-api-token-here"

# The client skips the authenticate() call
result = client.query_database(
    "SELECT version()"
)
print(result)
```

### JavaScript with API Token

In the following example, the JavaScript client uses a
pre-configured API token:

```javascript
const client = new PgEdgeMCPClient(
    "http://localhost:8080"
);
client.sessionToken = "your-api-token-here";

// The client skips the authenticate() call
const result = await client.queryDatabase(
    "SELECT version()"
);
console.log(result);
```

API tokens do not expire unless the administrator removes
the token from the configuration. The server reloads
token files automatically when the file changes.

## Error Handling Reference

The MCP server returns errors at two levels: HTTP status
codes and JSON-RPC error objects. This section documents
both error types.

### HTTP Status Codes

The server returns the following HTTP status codes:

- `401 Unauthorized` indicates a missing, invalid, or
  expired token. Re-authenticate to resolve this error.
- `400 Bad Request` indicates a malformed JSON-RPC
  payload. Verify the request format matches the spec.
- `404 Not Found` indicates the endpoint does not exist.
  Check that the URL path is correct.
- `500 Internal Error` indicates a server-side failure.
  Check the server logs for diagnostic details.

### JSON-RPC Error Codes

Tool calls may return JSON-RPC errors in the response body
instead of an HTTP error status. The following table lists
common JSON-RPC error codes:

| Code | Message | Cause |
|------|---------|-------|
| -32600 | Invalid Request | The JSON-RPC request is not valid. |
| -32601 | Method not found | The requested method does not exist. |
| -32602 | Invalid params | The tool parameters are missing or wrong. |
| -32603 | Internal error | The tool execution failed on the server. |

In the following example, a JSON-RPC error response
indicates that a tool was not found:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "error": {
        "code": -32601,
        "message": "Method not found"
    }
}
```

### Tool Execution Errors

Tool-level errors appear inside the `result` content when
the tool itself fails. The MCP server wraps these errors
in the standard content format.

In the following example, a query tool returns an error
for an invalid SQL statement:

```json
{
    "jsonrpc": "2.0",
    "id": 3,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "Error: relation \"nonexistent\" does not exist"
            }
        ],
        "isError": true
    }
}
```

## See Also

The following resources provide additional information
about the MCP server API and client development:

- The [API Reference](api-reference.md) documents all
  endpoints and response formats.
- The [Authentication Guide](../guide/authentication.md)
  explains token types and security configuration.
- The [Building Chat Clients](building-chat-clients.md)
  guide covers LLM-powered chatbot architecture.
- The [Tools Reference](../reference/tools.md) describes
  all available MCP tools and their parameters.
