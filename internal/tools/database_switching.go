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

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// ListDatabaseConnectionsTool creates a tool for listing available database connections
// This tool is only available when llm_connection_selection is enabled
func ListDatabaseConnectionsTool(
	clientManager *database.ClientManager,
	accessChecker *auth.DatabaseAccessChecker,
	cfg *config.Config,
) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "list_database_connections",
			Description: `List available database connections that you can switch to.

Returns a list of database connections configured for this session, along with
the currently active database. Use this to discover what databases are available
before using select_database_connection to switch.

Each database entry includes:
- name: The connection name (use this with select_database_connection)
- database: The PostgreSQL database name
- host: Database server hostname
- port: Database server port number
- allow_writes: Whether write operations are permitted

The response includes which database is currently active.`,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
				Required:   []string{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Extract context from args (injected by registry)
			ctx := extractContextFromArgs(args)

			// Get token hash for session identification
			tokenHash := auth.GetTokenHashFromContext(ctx)
			if tokenHash == "" {
				// STDIO mode uses "default" as the session key
				tokenHash = "default"
			}

			// Get all database configs
			allConfigs := clientManager.GetDatabaseConfigs()

			// Filter by user access control first
			var accessibleConfigs []config.NamedDatabaseConfig
			if accessChecker != nil {
				accessibleConfigs = accessChecker.GetAccessibleDatabases(ctx, allConfigs)
			} else {
				accessibleConfigs = allConfigs
			}

			// Further filter by LLM accessibility (allow_llm_switching)
			var llmAccessible []config.NamedDatabaseConfig
			for _, dbCfg := range accessibleConfigs {
				if dbCfg.IsAllowedForLLMSwitching() {
					llmAccessible = append(llmAccessible, dbCfg)
				}
			}

			// Get current database
			current := clientManager.GetCurrentDatabase(tokenHash)
			if current == "" {
				current = clientManager.GetDefaultDatabaseName()
			}

			// Ensure current database is in the accessible list (don't leak inaccessible db names)
			currentInList := false
			for _, db := range llmAccessible {
				if db.Name == current {
					currentInList = true
					break
				}
			}
			if !currentInList {
				// Don't report a different current DB than the session uses
				// Set to empty rather than misrepresenting the actual state
				current = ""
			}

			// Build response
			databases := make([]map[string]interface{}, 0, len(llmAccessible))
			for _, db := range llmAccessible {
				databases = append(databases, map[string]interface{}{
					"name":         db.Name,
					"database":     db.Database,
					"host":         db.Host,
					"port":         db.Port,
					"allow_writes": db.AllowWrites,
				})
			}

			result := map[string]interface{}{
				"databases": databases,
				"current":   current,
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to marshal response: %v", err))
			}

			return mcp.NewToolSuccess(string(jsonBytes))
		},
	}
}

// SelectDatabaseConnectionTool creates a tool for switching database connections
// This tool is only available when llm_connection_selection is enabled
func SelectDatabaseConnectionTool(
	clientManager *database.ClientManager,
	accessChecker *auth.DatabaseAccessChecker,
	cfg *config.Config,
) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "select_database_connection",
			Description: `Switch to a different database connection for subsequent queries.

Use list_database_connections first to see available options. After switching,
all subsequent database tools (query_database, get_schema_info, etc.) will
operate on the newly selected database.

IMPORTANT: Switching databases may change available schemas, tables, and
permissions. Consider re-examining the schema after switching.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the database connection to switch to (from list_database_connections)",
					},
				},
				Required: []string{"name"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Extract context from args (injected by registry)
			ctx := extractContextFromArgs(args)

			// Get the database name parameter
			name, ok := args["name"].(string)
			if !ok || name == "" {
				return mcp.NewToolError("Missing or invalid 'name' parameter")
			}

			// Get token hash for session identification
			tokenHash := auth.GetTokenHashFromContext(ctx)
			if tokenHash == "" {
				// STDIO mode uses "default" as the session key
				tokenHash = "default"
			}

			// Get database config
			// Use consistent error message to prevent information disclosure
			// (don't reveal whether database exists but is inaccessible)
			dbConfig := cfg.GetDatabaseByName(name)
			if dbConfig == nil {
				return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
			}

			// Check user access control
			if accessChecker != nil {
				// For API tokens, enforce database binding
				if auth.IsAPITokenFromContext(ctx) {
					boundDB := accessChecker.GetBoundDatabase(ctx)
					if boundDB != "" && boundDB != name {
						return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
					}
				} else if !accessChecker.CanAccessDatabase(ctx, dbConfig) {
					// For session users, check available_to_users
					return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
				}
			}

			// Check LLM accessibility (allow_llm_switching)
			if !dbConfig.IsAllowedForLLMSwitching() {
				return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
			}

			// Perform the switch
			if err := clientManager.SetCurrentDatabase(tokenHash, name); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to switch database: %v", err))
			}

			// Build success response
			result := map[string]interface{}{
				"success":      true,
				"message":      fmt.Sprintf("Switched to database: %s", name),
				"current":      name,
				"database":     dbConfig.Database,
				"host":         dbConfig.Host,
				"allow_writes": dbConfig.AllowWrites,
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to marshal response: %v", err))
			}

			return mcp.NewToolSuccess(string(jsonBytes))
		},
	}
}

// extractContextFromArgs extracts context.Context from tool args
func extractContextFromArgs(args map[string]interface{}) context.Context {
	if ctxRaw, ok := args["__context"]; ok {
		if ctx, ok := ctxRaw.(context.Context); ok {
			return ctx
		}
	}
	return context.Background()
}
