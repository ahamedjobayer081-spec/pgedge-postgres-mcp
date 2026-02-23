/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
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
	"github.com/jackc/pgx/v5/pgxpool"

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

// isLanguageAllowed checks if a PL language is allowed for execution
func (e *CustomToolExecutor) isLanguageAllowed(language string) bool {
	lang := strings.ToLower(language)
	return e.allowedPLLanguages[lang] || e.allowedPLLanguages["*"]
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
			_ = tx.Rollback(context.Background()) //nolint:errcheck // best-effort rollback
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
//
// The DO block and result retrieval must run in the same transaction because
// set_config(key, value, true) sets the value for the current transaction only.
// Using separate pool calls could assign different connections, losing the result.
func (e *CustomToolExecutor) executePLDOTool(ctx context.Context, def definitions.ToolDefinition, args map[string]interface{}) (mcp.ToolResponse, error) {
	// Check if the language is allowed first (security check before db access)
	if !e.isLanguageAllowed(def.Language) {
		return mcp.NewToolError(fmt.Sprintf("PL language '%s' is not allowed for this database connection", def.Language))
	}

	if e.dbClient == nil {
		return mcp.NewToolError("database client not available")
	}

	pool := e.dbClient.GetPoolFor(e.dbClient.GetDefaultConnection())
	if pool == nil {
		return mcp.NewToolError("connection pool not available")
	}

	// Build the DO block SQL
	wrappedCode := e.wrapPLDOCode(def.Language, def.Code, args)
	doSQL := fmt.Sprintf("DO $mcp_custom_tool$\n%s\n$mcp_custom_tool$ LANGUAGE %s;", wrappedCode, def.Language)

	// Execute the DO block and retrieve the result in a single transaction
	// so that the transaction-local set_config value is visible to the
	// subsequent current_setting query.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to begin transaction: %v", err))
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background()) //nolint:errcheck // best-effort rollback
		}
	}()

	// Set read-only unless writes are explicitly allowed
	if !e.dbClient.AllowWrites() {
		if _, err := tx.Exec(ctx, "SET TRANSACTION READ ONLY"); err != nil {
			return mcp.NewToolError(fmt.Sprintf("failed to set transaction read-only: %v", err))
		}
	}

	// nosemgrep: go.lang.security.audit.sqli.tainted-sql-string
	// This tool is explicitly designed to execute user-provided PL code
	if _, err := tx.Exec(ctx, doSQL); err != nil {
		return mcp.NewToolError(fmt.Sprintf("DO block execution failed: %v", err))
	}

	result, resultErr := e.retrievePLDOResult(ctx, tx)
	if resultErr != nil {
		return result, resultErr
	}

	if err := tx.Commit(ctx); err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to commit transaction: %v", err))
	}
	committed = true

	return result, nil
}

// retrievePLDOResult retrieves the result from a PL/DO block execution
// using the provided transaction to ensure the set_config value is visible.
func (e *CustomToolExecutor) retrievePLDOResult(ctx context.Context, tx pgx.Tx) (mcp.ToolResponse, error) {
	var resultStr *string
	query := fmt.Sprintf("SELECT current_setting('%s', true)", mcpResultConfigKey)
	if err := tx.QueryRow(ctx, query).Scan(&resultStr); err != nil || resultStr == nil || *resultStr == "" {
		return mcp.NewToolSuccess("Tool executed successfully")
	}

	return e.formatJSONResult(*resultStr)
}

// formatJSONResult formats a result string, attempting JSON pretty-printing
func (e *CustomToolExecutor) formatJSONResult(resultStr string) (mcp.ToolResponse, error) {
	var jsonResult interface{}
	if err := json.Unmarshal([]byte(resultStr), &jsonResult); err == nil {
		formatted, _ := json.MarshalIndent(jsonResult, "", "  ") //nolint:errcheck // fallback to original on error
		return mcp.NewToolSuccess(string(formatted))
	}
	return mcp.NewToolSuccess(resultStr)
}

// plFuncSQL holds the SQL statements for PL function execution
type plFuncSQL struct {
	createSQL string
	callSQL   string
	dropSQL   string
	argsJSON  string
}

// buildPLFuncSQL builds the SQL statements for PL function execution
func (e *CustomToolExecutor) buildPLFuncSQL(def definitions.ToolDefinition, args map[string]interface{}) (*plFuncSQL, error) {
	funcName := fmt.Sprintf("_mcp_custom_tool_%s", strings.ReplaceAll(uuid.New().String(), "-", ""))
	wrappedCode := e.wrapPLFuncCode(def.Language, def.Code, args)

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize arguments: %w", err)
	}

	createSQL := fmt.Sprintf(
		"CREATE OR REPLACE FUNCTION %s(args jsonb) RETURNS %s AS $mcp_func$\n%s\n$mcp_func$ LANGUAGE %s;",
		funcName, def.Returns, wrappedCode, def.Language,
	)

	callSQL := fmt.Sprintf("SELECT %s($1::jsonb);", funcName)
	if strings.HasPrefix(strings.ToUpper(def.Returns), "TABLE") {
		callSQL = fmt.Sprintf("SELECT * FROM %s($1::jsonb);", funcName)
	}

	return &plFuncSQL{
		createSQL: createSQL,
		callSQL:   callSQL,
		dropSQL:   fmt.Sprintf("DROP FUNCTION IF EXISTS %s(jsonb);", funcName),
		argsJSON:  string(argsJSON),
	}, nil
}

// executePLFuncTool executes a PL function custom tool (creates temp function, calls it, drops it)
func (e *CustomToolExecutor) executePLFuncTool(ctx context.Context, def definitions.ToolDefinition, args map[string]interface{}) (mcp.ToolResponse, error) {
	if !e.isLanguageAllowed(def.Language) {
		return mcp.NewToolError(fmt.Sprintf("PL language '%s' is not allowed for this database connection", def.Language))
	}

	if e.dbClient == nil {
		return mcp.NewToolError("database client not available")
	}

	pool := e.dbClient.GetPoolFor(e.dbClient.GetDefaultConnection())
	if pool == nil {
		return mcp.NewToolError("connection pool not available")
	}

	sqlStmts, err := e.buildPLFuncSQL(def, args)
	if err != nil {
		return mcp.NewToolError(err.Error())
	}

	return e.executePLFuncInTransaction(ctx, pool, sqlStmts)
}

// executePLFuncInTransaction executes the PL function within a transaction
func (e *CustomToolExecutor) executePLFuncInTransaction(ctx context.Context, pool *pgxpool.Pool, sql *plFuncSQL) (mcp.ToolResponse, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to begin transaction: %v", err))
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background()) //nolint:errcheck // best-effort rollback
		}
	}()

	// nosemgrep: go.lang.security.audit.sqli.tainted-sql-string
	// This tool is explicitly designed to execute user-provided PL functions
	if _, err := tx.Exec(ctx, sql.createSQL); err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to create function: %v", err))
	}

	// nosemgrep: go.lang.security.audit.sqli.tainted-sql-string
	rows, err := tx.Query(ctx, sql.callSQL, sql.argsJSON)
	if err != nil {
		// nosemgrep: go.lang.security.audit.sqli.tainted-sql-string
		_, _ = tx.Exec(ctx, sql.dropSQL) //nolint:errcheck // best-effort cleanup
		return mcp.NewToolError(fmt.Sprintf("function execution failed: %v", err))
	}

	result, err := e.formatQueryResults(rows)
	rows.Close()
	if err != nil {
		_, _ = tx.Exec(ctx, sql.dropSQL) //nolint:errcheck // best-effort cleanup
		return mcp.NewToolError(fmt.Sprintf("failed to format results: %v", err))
	}

	// nosemgrep: go.lang.security.audit.sqli.tainted-sql-string
	_, _ = tx.Exec(ctx, sql.dropSQL) //nolint:errcheck // best-effort cleanup

	if err := tx.Commit(ctx); err != nil {
		return mcp.NewToolError(fmt.Sprintf("failed to commit transaction: %v", err))
	}
	committed = true

	return mcp.NewToolSuccess(result)
}

// findMaxParamPlaceholder finds the highest $N placeholder in SQL (up to 100)
func findMaxParamPlaceholder(sql string) int {
	maxParam := 0
	for i := 1; i <= 100; i++ {
		if strings.Contains(sql, fmt.Sprintf("$%d", i)) {
			maxParam = i
		}
	}
	return maxParam
}

// buildOrderedProperties returns property names ordered by required first, then remaining
func buildOrderedProperties(schema definitions.ToolInputSchema) []string {
	requiredSet := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		requiredSet[name] = true
	}

	orderedProps := make([]string, 0, len(schema.Properties))
	orderedProps = append(orderedProps, schema.Required...)

	for name := range schema.Properties {
		if !requiredSet[name] {
			orderedProps = append(orderedProps, name)
		}
	}
	return orderedProps
}

// getParamValue returns the value for a parameter, checking args then defaults
func getParamValue(propName string, schema definitions.ToolInputSchema, args map[string]interface{}) interface{} {
	if val, ok := args[propName]; ok {
		return val
	}
	if prop, exists := schema.Properties[propName]; exists && prop.Default != nil {
		return prop.Default
	}
	return nil
}

// buildSQLParams extracts parameters from args in order based on $1, $2, etc. placeholders
func (e *CustomToolExecutor) buildSQLParams(sql string, schema definitions.ToolInputSchema, args map[string]interface{}) ([]interface{}, error) {
	maxParam := findMaxParamPlaceholder(sql)
	if maxParam == 0 {
		return nil, nil
	}

	orderedProps := buildOrderedProperties(schema)
	params := make([]interface{}, maxParam)

	for i := 0; i < maxParam && i < len(orderedProps); i++ {
		params[i] = getParamValue(orderedProps[i], schema, args)
	}

	return params, nil
}

// wrapPLDOCode wraps user code with args injection and mcp_return helper for DO blocks
// The mcp_return() function uses set_config to store results in a session variable,
// which works in read-only transactions and doesn't pollute server logs.
func (e *CustomToolExecutor) wrapPLDOCode(language, code string, args map[string]interface{}) string {
	argsJSON, _ := json.Marshal(args) //nolint:errcheck // args is always serializable
	argsStr := string(argsJSON)

	switch strings.ToLower(language) {
	case "plpython3u", "plpythonu":
		return wrapPLDOPython(argsStr, code)
	case "plpgsql":
		return wrapPLDOPgSQL(argsStr, code)
	case "plv8":
		return wrapPLDOV8(argsStr, code)
	case "plperl":
		return wrapPLDOPerl(argsStr, code)
	case "plperlu":
		return wrapPLDOPerlu(argsStr, code)
	default:
		return fmt.Sprintf("-- args: %s\n%s", argsStr, code)
	}
}

func wrapPLDOPython(argsJSON, code string) string {
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
`, argsJSON, mcpResultConfigKey, code)
}

func wrapPLDOPgSQL(argsJSON, code string) string {
	// Use dollar-quoting ($mcp_args$...$mcp_args$) for the JSON args to avoid
	// escaping issues with backslashes or quotes in JSON values.
	// Note: %q would add Go-style backslash escapes that PostgreSQL doesn't understand.
	return fmt.Sprintf(`
<<mcp_block>>
DECLARE
    args jsonb := $mcp_args$%s$mcp_args$::jsonb;
    result jsonb;
BEGIN
    -- To return a result, use: PERFORM set_config('%s', result::text, true);
%s
END mcp_block;
`, argsJSON, mcpResultConfigKey, code)
}

func wrapPLDOV8(argsJSON, code string) string {
	return fmt.Sprintf(`
var args = %s;

function mcp_return(result) {
    var val = (typeof result === 'string') ? result : JSON.stringify(result);
    plv8.execute("SELECT set_config('%s', $1, true)", [val]);
}

%s
`, argsJSON, mcpResultConfigKey, code)
}

func wrapPLDOPerl(argsJSON, code string) string {
	return fmt.Sprintf(`
my $args_json = %q;
my $rv = spi_exec_query("SELECT key, value FROM jsonb_each_text(" . quote_literal($args_json) . "::jsonb)");
my $args = {};
foreach my $row (@{$rv->{rows}}) {
    $args->{$row->{key}} = $row->{value};
}

sub _to_json {
    my ($d) = @_;
    return 'null' unless defined $d;
    if (ref($d) eq 'HASH') {
        my @p;
        for my $k (sort keys %%{$d}) {
            my $ek = $k;
            $ek =~ s/\\/\\\\/g;
            $ek =~ s/"/\\"/g;
            push @p, '"' . $ek . '":' . _to_json($d->{$k});
        }
        return '{' . join(',', @p) . '}';
    }
    if (ref($d) eq 'ARRAY') {
        return '[' . join(',', map { _to_json($_) } @{$d}) . ']';
    }
    if ($d =~ /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$/) {
        return $d;
    }
    my $s = $d;
    $s =~ s/\\/\\\\/g;
    $s =~ s/"/\\"/g;
    $s =~ s/\n/\\n/g;
    $s =~ s/\r/\\r/g;
    $s =~ s/\t/\\t/g;
    return '"' . $s . '"';
}

sub mcp_return {
    my ($result) = @_;
    my $val = ref($result) ? _to_json($result) : $result;
    spi_exec_query("SELECT set_config('%s', " . quote_literal($val) . ", true)");
}

%s
`, argsJSON, mcpResultConfigKey, code)
}

func wrapPLDOPerlu(argsJSON, code string) string {
	return fmt.Sprintf(`
use JSON;
my $args = decode_json(%q);

sub mcp_return {
    my ($result) = @_;
    my $val = ref($result) ? encode_json($result) : $result;
    spi_exec_query("SELECT set_config('%s', " . quote_literal($val) . ", true)");
}

%s
`, argsJSON, mcpResultConfigKey, code)
}

// wrapPLFuncCode wraps user code for function creation
func (e *CustomToolExecutor) wrapPLFuncCode(language, code string, args map[string]interface{}) string {
	switch strings.ToLower(language) {
	case "plpython3u", "plpythonu":
		// PL/Python sets function parameters as global variables and wraps
		// the body in a parameterless Python function. Without "global args",
		// the assignment makes Python treat args as an uninitialized local,
		// causing UnboundLocalError when json.loads(args) reads it.
		return fmt.Sprintf(`
import json
global args
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

	case "plperlu":
		return fmt.Sprintf(`
use JSON;
my $args_json = $_[0];
my $args = decode_json($args_json);

%s
`, code)

	case "plperl":
		return fmt.Sprintf(`
my $args_json = $_[0];
my $rv = spi_exec_query("SELECT key, value FROM jsonb_each_text(" . quote_literal($args_json) . "::jsonb)");
my $args = {};
foreach my $row (@{$rv->{rows}}) {
    $args->{$row->{key}} = $row->{value};
}

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
