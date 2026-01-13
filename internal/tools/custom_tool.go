/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/mcp"
)

// mcpResultConfigKey is the session config key used to store pl-do tool results
// We use set_config/current_setting which works in read-only transactions
const mcpResultConfigKey = "mcp.tool_result"

// CustomToolExecutor handles execution of user-defined custom tools
type CustomToolExecutor struct {
	dbClient           *database.Client
	allowedPLLanguages map[string]bool
	defaultTimeout     time.Duration
}

// NewCustomToolExecutor creates a new custom tool executor
func NewCustomToolExecutor(dbClient *database.Client, allowedLanguages []string) *CustomToolExecutor {
	langMap := make(map[string]bool)
	for _, lang := range allowedLanguages {
		langMap[strings.ToLower(lang)] = true
	}

	return &CustomToolExecutor{
		dbClient:           dbClient,
		allowedPLLanguages: langMap,
		defaultTimeout:     30 * time.Second,
	}
}

// CreateTool creates an MCP Tool from a ToolDefinition
func (e *CustomToolExecutor) CreateTool(def definitions.ToolDefinition) Tool {
	// Convert the input schema to MCP format
	properties := make(map[string]interface{})
	for name, prop := range def.InputSchema.Properties {
		properties[name] = convertPropertyToMCP(prop)
	}

	inputSchema := mcp.InputSchema{
		Type:       "object",
		Properties: properties,
		Required:   def.InputSchema.Required,
	}

	// Build description with type indicator
	description := def.Description
	if description == "" {
		description = fmt.Sprintf("Custom %s tool", def.Type)
	}

	return Tool{
		Definition: mcp.Tool{
			Name:        def.Name,
			Description: description,
			InputSchema: inputSchema,
		},
		Handler: e.createHandler(def),
	}
}

// convertPropertyToMCP converts a ToolProperty to MCP-compatible format
func convertPropertyToMCP(prop definitions.ToolProperty) map[string]interface{} {
	result := map[string]interface{}{
		"type": prop.Type,
	}
	if prop.Description != "" {
		result["description"] = prop.Description
	}
	if prop.Default != nil {
		result["default"] = prop.Default
	}
	if len(prop.Enum) > 0 {
		result["enum"] = prop.Enum
	}
	if prop.Items != nil {
		result["items"] = convertPropertyToMCP(*prop.Items)
	}
	return result
}

// createHandler creates the handler function for a custom tool
func (e *CustomToolExecutor) createHandler(def definitions.ToolDefinition) Handler {
	return func(args map[string]interface{}) (mcp.ToolResponse, error) {
		// Parse timeout if specified
		timeout := e.defaultTimeout
		if def.Timeout != "" {
			if parsed, err := time.ParseDuration(def.Timeout); err == nil {
				timeout = parsed
			}
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Execute based on type
		switch def.Type {
		case "sql":
			return e.executeSQLTool(ctx, def, args)
		case "pl-do":
			return e.executePLDOTool(ctx, def, args)
		case "pl-func":
			return e.executePLFuncTool(ctx, def, args)
		default:
			return mcp.NewToolError(fmt.Sprintf("unsupported tool type: %s", def.Type))
		}
	}
}

// executeSQLTool executes a SQL-based custom tool
func (e *CustomToolExecutor) executeSQLTool(ctx context.Context, def definitions.ToolDefinition, args map[string]interface{}) (mcp.ToolResponse, error) {
	if e.dbClient == nil {
		return mcp.NewToolError("database client not available")
	}

	// Build ordered parameter list based on $1, $2, etc. in the SQL
	params, err := e.buildSQLParams(def.SQL, def.InputSchema, args)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("parameter error: %v", err))
	}

	// Get connection pool
	connStr := e.dbClient.GetDefaultConnection()
	pool := e.dbClient.GetPoolFor(connStr)
	if pool == nil {
		return mcp.NewToolError("connection pool not available")
	}

	// Execute in a read-only transaction (unless writes are allowed)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to begin transaction: %v", err))
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Set read-only unless writes are explicitly allowed
	if !e.dbClient.AllowWrites() {
		if _, err := tx.Exec(ctx, "SET TRANSACTION READ ONLY"); err != nil {
			return mcp.NewToolError(fmt.Sprintf("failed to set transaction read-only: %v", err))
		}
	}

	// Execute the query
	rows, err := tx.Query(ctx, def.SQL, params...)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("query execution failed: %v", err))
	}
	defer rows.Close()

	// Format results
	result, err := e.formatQueryResults(rows)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to format results: %v", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to commit transaction: %v", err))
	}
	committed = true

	return mcp.NewToolSuccess(result)
}

// executePLDOTool executes a PL DO block custom tool
// Uses PostgreSQL session variables (set_config/current_setting) to return results,
// which works in read-only transactions and doesn't pollute server logs.
// The tool code should call mcp_return(result) to emit output.
func (e *CustomToolExecutor) executePLDOTool(ctx context.Context, def definitions.ToolDefinition, args map[string]interface{}) (mcp.ToolResponse, error) {
	// Check if the language is allowed first (security check before db access)
	lang := strings.ToLower(def.Language)
	if !e.allowedPLLanguages[lang] && !e.allowedPLLanguages["*"] {
		return mcp.NewToolError(fmt.Sprintf("PL language '%s' is not allowed for this database connection", def.Language))
	}

	if e.dbClient == nil {
		return mcp.NewToolError("database client not available")
	}

	// Get connection pool
	connStr := e.dbClient.GetDefaultConnection()
	pool := e.dbClient.GetPoolFor(connStr)
	if pool == nil {
		return mcp.NewToolError("connection pool not available")
	}

	// Wrap the user's code with args injection and result helper
	wrappedCode := e.wrapPLDOCode(def.Language, def.Code, args)

	// Build the DO block
	doSQL := fmt.Sprintf("DO $mcp_custom_tool$\n%s\n$mcp_custom_tool$ LANGUAGE %s;", wrappedCode, def.Language)

	// Execute the DO block
	_, err := pool.Exec(ctx, doSQL)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("DO block execution failed: %v", err))
	}

	// Retrieve the result from the session variable
	var resultStr *string
	err = pool.QueryRow(ctx, fmt.Sprintf("SELECT current_setting('%s', true)", mcpResultConfigKey)).Scan(&resultStr)
	if err != nil {
		// If we can't get the setting, it means no result was set
		return mcp.NewToolSuccess("Tool executed successfully")
	}

	if resultStr == nil || *resultStr == "" {
		return mcp.NewToolSuccess("Tool executed successfully")
	}

	// Try to parse as JSON for nice formatting
	var jsonResult interface{}
	if err := json.Unmarshal([]byte(*resultStr), &jsonResult); err == nil {
		formatted, _ := json.MarshalIndent(jsonResult, "", "  ")
		return mcp.NewToolSuccess(string(formatted))
	}

	return mcp.NewToolSuccess(*resultStr)
}

// executePLFuncTool executes a PL function custom tool (creates temp function, calls it, drops it)
func (e *CustomToolExecutor) executePLFuncTool(ctx context.Context, def definitions.ToolDefinition, args map[string]interface{}) (mcp.ToolResponse, error) {
	// Check if the language is allowed first (security check before db access)
	lang := strings.ToLower(def.Language)
	if !e.allowedPLLanguages[lang] && !e.allowedPLLanguages["*"] {
		return mcp.NewToolError(fmt.Sprintf("PL language '%s' is not allowed for this database connection", def.Language))
	}

	if e.dbClient == nil {
		return mcp.NewToolError("database client not available")
	}

	// Get connection pool
	connStr := e.dbClient.GetDefaultConnection()
	pool := e.dbClient.GetPoolFor(connStr)
	if pool == nil {
		return mcp.NewToolError("connection pool not available")
	}

	// Generate unique function name
	funcName := fmt.Sprintf("_mcp_custom_tool_%s", strings.ReplaceAll(uuid.New().String(), "-", ""))

	// Wrap the user's code
	wrappedCode := e.wrapPLFuncCode(def.Language, def.Code, args)

	// Build CREATE FUNCTION statement
	createSQL := fmt.Sprintf(
		"CREATE OR REPLACE FUNCTION %s(args jsonb) RETURNS %s AS $mcp_func$\n%s\n$mcp_func$ LANGUAGE %s;",
		funcName, def.Returns, wrappedCode, def.Language,
	)

	// Build function call
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to serialize arguments: %v", err))
	}

	var callSQL string
	if strings.HasPrefix(strings.ToUpper(def.Returns), "TABLE") {
		// For table-returning functions, use SELECT * FROM
		callSQL = fmt.Sprintf("SELECT * FROM %s($1::jsonb);", funcName)
	} else {
		// For scalar returns, use SELECT
		callSQL = fmt.Sprintf("SELECT %s($1::jsonb);", funcName)
	}

	// Build DROP FUNCTION statement
	dropSQL := fmt.Sprintf("DROP FUNCTION IF EXISTS %s(jsonb);", funcName)

	// Execute in a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to begin transaction: %v", err))
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Create the function
	if _, err := tx.Exec(ctx, createSQL); err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to create function: %v", err))
	}

	// Call the function
	rows, err := tx.Query(ctx, callSQL, string(argsJSON))
	if err != nil {
		// Try to drop the function before returning error
		_, _ = tx.Exec(ctx, dropSQL)
		return mcp.NewToolError(fmt.Sprintf("function execution failed: %v", err))
	}

	// Format results
	result, err := e.formatQueryResults(rows)
	rows.Close()
	if err != nil {
		_, _ = tx.Exec(ctx, dropSQL)
		return mcp.NewToolError(fmt.Sprintf("failed to format results: %v", err))
	}

	// Drop the function
	if _, err := tx.Exec(ctx, dropSQL); err != nil {
		// Log but don't fail - function will be cleaned up eventually
		fmt.Printf("Warning: failed to drop temp function %s: %v\n", funcName, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to commit transaction: %v", err))
	}
	committed = true

	return mcp.NewToolSuccess(result)
}

// buildSQLParams extracts parameters from args in order based on $1, $2, etc. placeholders
func (e *CustomToolExecutor) buildSQLParams(sql string, schema definitions.ToolInputSchema, args map[string]interface{}) ([]interface{}, error) {
	// Find all $N placeholders and determine the maximum N
	maxParam := 0
	for i := 1; i <= 100; i++ {
		placeholder := fmt.Sprintf("$%d", i)
		if strings.Contains(sql, placeholder) {
			maxParam = i
		}
	}

	if maxParam == 0 {
		return nil, nil // No parameters needed
	}

	// Build ordered property list from schema
	// The order in 'required' + remaining properties determines $1, $2, etc.
	orderedProps := make([]string, 0, len(schema.Properties))

	// First add required properties in order
	for _, name := range schema.Required {
		orderedProps = append(orderedProps, name)
	}

	// Then add remaining properties
	for name := range schema.Properties {
		found := false
		for _, req := range schema.Required {
			if req == name {
				found = true
				break
			}
		}
		if !found {
			orderedProps = append(orderedProps, name)
		}
	}

	// Build parameter slice
	params := make([]interface{}, maxParam)
	for i := 0; i < maxParam && i < len(orderedProps); i++ {
		propName := orderedProps[i]
		if val, ok := args[propName]; ok {
			params[i] = val
		} else if prop, exists := schema.Properties[propName]; exists && prop.Default != nil {
			params[i] = prop.Default
		} else {
			params[i] = nil
		}
	}

	return params, nil
}

// wrapPLDOCode wraps user code with args injection and mcp_return helper for DO blocks
// The mcp_return() function uses set_config to store results in a session variable,
// which works in read-only transactions and doesn't pollute server logs.
func (e *CustomToolExecutor) wrapPLDOCode(language, code string, args map[string]interface{}) string {
	argsJSON, _ := json.Marshal(args)

	switch strings.ToLower(language) {
	case "plpython3u", "plpythonu":
		// For plpython3u, provide mcp_return() function that uses set_config
		return fmt.Sprintf(`
import json

args = json.loads(%q)

def mcp_return(result):
    """Return a result from this tool. Result will be JSON-encoded."""
    if isinstance(result, str):
        val = result
    else:
        val = json.dumps(result)
    plpy.execute("SELECT set_config('%s', $1, true)", [val])

%s
`, string(argsJSON), mcpResultConfigKey, code)

	case "plpgsql":
		// For plpgsql, the user calls: PERFORM mcp_return(result);
		// We create a local procedure-like pattern using set_config
		return fmt.Sprintf(`
<<mcp_block>>
DECLARE
    args jsonb := %q::jsonb;
    result jsonb;
BEGIN
    -- To return a result, use: PERFORM set_config('%s', result::text, true);
%s
END mcp_block;
`, string(argsJSON), mcpResultConfigKey, code)

	case "plv8":
		return fmt.Sprintf(`
var args = %s;

function mcp_return(result) {
    var val = (typeof result === 'string') ? result : JSON.stringify(result);
    plv8.execute("SELECT set_config('%s', $1, true)", [val]);
}

%s
`, string(argsJSON), mcpResultConfigKey, code)

	case "plperl", "plperlu":
		return fmt.Sprintf(`
use JSON;
my $args = decode_json(%q);

sub mcp_return {
    my ($result) = @_;
    my $val = ref($result) ? encode_json($result) : $result;
    spi_exec_query("SELECT set_config('%s', " . quote_literal($val) . ", true)");
}

%s
`, string(argsJSON), mcpResultConfigKey, code)

	default:
		// For unknown languages, just prepend args as JSON comment
		return fmt.Sprintf("-- args: %s\n%s", string(argsJSON), code)
	}
}

// wrapPLFuncCode wraps user code for function creation
func (e *CustomToolExecutor) wrapPLFuncCode(language, code string, args map[string]interface{}) string {
	switch strings.ToLower(language) {
	case "plpython3u", "plpythonu":
		return fmt.Sprintf(`
import json
args = json.loads(args)

%s
`, code)

	case "plpgsql":
		// For plpgsql, the args parameter is already available as 'args'
		return code

	case "plv8":
		return fmt.Sprintf(`
var argsObj = JSON.parse(args);
var args = argsObj;

%s
`, code)

	case "plperl", "plperlu":
		return fmt.Sprintf(`
use JSON;
my $args_json = $_[0];
my $args = decode_json($args_json);

%s
`, code)

	default:
		return code
	}
}

// formatQueryResults formats query results as a readable string
func (e *CustomToolExecutor) formatQueryResults(rows pgx.Rows) (string, error) {
	// Get column names
	fieldDescriptions := rows.FieldDescriptions()
	var columnNames []string
	for _, fd := range fieldDescriptions {
		columnNames = append(columnNames, string(fd.Name))
	}

	// Collect all rows
	var results [][]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return "", fmt.Errorf("error reading row: %w", err)
		}
		results = append(results, values)
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating rows: %w", err)
	}

	// Format as TSV
	return FormatResultsAsTSV(columnNames, results), nil
}
