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
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
)

// Helper to create a test config with databases
func createTestDatabaseConfigs() []config.NamedDatabaseConfig {
	trueVal := true
	falseVal := false
	return []config.NamedDatabaseConfig{
		{
			Name:              "db1",
			Host:              "localhost",
			Port:              5432,
			Database:          "testdb1",
			User:              "user1",
			AllowWrites:       false,
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "db2",
			Host:              "remotehost",
			Port:              5433,
			Database:          "testdb2",
			User:              "user2",
			AllowWrites:       true,
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "db3-no-llm",
			Host:              "localhost",
			Port:              5432,
			Database:          "testdb3",
			User:              "user3",
			AllowWrites:       false,
			AllowLLMSwitching: &falseVal, // LLM switching disabled
		},
	}
}

// TestListDatabaseConnectionsTool_Basic tests basic listing functionality
func TestListDatabaseConnectionsTool_Basic(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	// Verify tool definition
	if tool.Definition.Name != "list_database_connections" {
		t.Errorf("Expected tool name 'list_database_connections', got %q", tool.Definition.Name)
	}

	// Execute the tool
	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if response.IsError {
		t.Fatalf("Expected success response, got error: %v", response.Content)
	}

	// Parse response
	var result struct {
		Databases []struct {
			Name        string `json:"name"`
			Database    string `json:"database"`
			Host        string `json:"host"`
			Port        int    `json:"port"`
			AllowWrites bool   `json:"allow_writes"`
		} `json:"databases"`
		Current string `json:"current"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have 2 databases (db3-no-llm is filtered out due to allow_llm_switching: false)
	if len(result.Databases) != 2 {
		t.Errorf("Expected 2 databases, got %d", len(result.Databases))
	}

	// Verify first database has all expected fields
	found := false
	for _, db := range result.Databases {
		if db.Name == "db1" {
			found = true
			if db.Database != "testdb1" {
				t.Errorf("Expected database 'testdb1', got %q", db.Database)
			}
			if db.Host != "localhost" {
				t.Errorf("Expected host 'localhost', got %q", db.Host)
			}
			if db.Port != 5432 {
				t.Errorf("Expected port 5432, got %d", db.Port)
			}
			if db.AllowWrites != false {
				t.Errorf("Expected allow_writes false, got %v", db.AllowWrites)
			}
		}
	}
	if !found {
		t.Error("Database 'db1' not found in response")
	}
}

// TestListDatabaseConnectionsTool_FiltersLLMSwitching tests allow_llm_switching filtering
func TestListDatabaseConnectionsTool_FiltersLLMSwitching(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []struct {
			Name string `json:"name"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// db3-no-llm should NOT be in the list
	for _, db := range result.Databases {
		if db.Name == "db3-no-llm" {
			t.Error("Database with allow_llm_switching: false should not be listed")
		}
	}
}

// TestListDatabaseConnectionsTool_EmptyConfig tests with no databases configured
func TestListDatabaseConnectionsTool_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Databases: []config.NamedDatabaseConfig{}}
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []interface{} `json:"databases"`
		Current   string        `json:"current"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Databases) != 0 {
		t.Errorf("Expected 0 databases, got %d", len(result.Databases))
	}
	if result.Current != "" {
		t.Errorf("Expected empty current database, got %q", result.Current)
	}
}

// TestSelectDatabaseConnectionTool_Success tests successful database switching
func TestSelectDatabaseConnectionTool_Success(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	// Verify tool definition
	if tool.Definition.Name != "select_database_connection" {
		t.Errorf("Expected tool name 'select_database_connection', got %q", tool.Definition.Name)
	}

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "db2",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if response.IsError {
		t.Fatalf("Expected success response, got error: %v", response.Content)
	}

	var result struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		Current     string `json:"current"`
		Database    string `json:"database"`
		Host        string `json:"host"`
		AllowWrites bool   `json:"allow_writes"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result.Success {
		t.Error("Expected success=true")
	}
	if result.Current != "db2" {
		t.Errorf("Expected current='db2', got %q", result.Current)
	}
	if result.Database != "testdb2" {
		t.Errorf("Expected database='testdb2', got %q", result.Database)
	}
	if result.Host != "remotehost" {
		t.Errorf("Expected host='remotehost', got %q", result.Host)
	}
	if result.AllowWrites != true {
		t.Error("Expected allow_writes=true")
	}
}

// TestSelectDatabaseConnectionTool_MissingName tests missing name parameter
func TestSelectDatabaseConnectionTool_MissingName(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for missing name parameter")
	}

	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Missing or invalid 'name' parameter") {
		t.Errorf("Expected 'Missing or invalid' error, got: %s", response.Content[0].Text)
	}
}

// TestSelectDatabaseConnectionTool_DatabaseNotFound tests non-existent database
func TestSelectDatabaseConnectionTool_DatabaseNotFound(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "nonexistent-db",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for non-existent database")
	}

	// Should use "Access denied" message (not "not found") to prevent information disclosure
	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Access denied") {
		t.Errorf("Expected 'Access denied' error to prevent info disclosure, got: %s", response.Content[0].Text)
	}
}

// TestSelectDatabaseConnectionTool_LLMSwitchingDisabled tests allow_llm_switching: false
func TestSelectDatabaseConnectionTool_LLMSwitchingDisabled(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "db3-no-llm", // This database has allow_llm_switching: false
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for database with LLM switching disabled")
	}

	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Access denied") {
		t.Errorf("Expected 'Access denied' error, got: %s", response.Content[0].Text)
	}
}

// TestExtractContextFromArgs tests context extraction helper
func TestExtractContextFromArgs(t *testing.T) {
	t.Run("with context", func(t *testing.T) {
		type testKey string
		ctx := context.WithValue(context.Background(), testKey("test"), "value")
		args := map[string]interface{}{
			"__context": ctx,
		}
		extracted := extractContextFromArgs(args)
		if extracted.Value(testKey("test")) != "value" {
			t.Error("Context not properly extracted")
		}
	})

	t.Run("without context", func(t *testing.T) {
		args := map[string]interface{}{}
		extracted := extractContextFromArgs(args)
		if extracted == nil {
			t.Error("Expected background context, got nil")
		}
	})

	t.Run("with invalid context type", func(t *testing.T) {
		args := map[string]interface{}{
			"__context": "not-a-context",
		}
		extracted := extractContextFromArgs(args)
		if extracted == nil {
			t.Error("Expected background context, got nil")
		}
	})
}

// TestListDatabaseConnectionsTool_HidesInaccessibleCurrent tests that current db is hidden if inaccessible
func TestListDatabaseConnectionsTool_HidesInaccessibleCurrent(t *testing.T) {
	// Create config with one LLM-accessible and one non-accessible database
	trueVal := true
	falseVal := false
	databases := []config.NamedDatabaseConfig{
		{
			Name:              "accessible-db",
			Host:              "localhost",
			Port:              5432,
			Database:          "accessible",
			User:              "user1",
			AllowLLMSwitching: &trueVal,
		},
		{
			Name:              "hidden-db",
			Host:              "localhost",
			Port:              5432,
			Database:          "hidden",
			User:              "user1",
			AllowLLMSwitching: &falseVal,
		},
	}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	// Set current database to the hidden one
	clientManager.SetCurrentDatabase("default", "hidden-db")

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Current string `json:"current"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Current should NOT be "hidden-db" since it's not accessible to LLM
	// It should be empty to avoid misrepresenting the actual session state
	if result.Current == "hidden-db" {
		t.Error("Current database should not reveal inaccessible database name")
	}
	if result.Current != "" {
		t.Errorf("Current should be empty when inaccessible to LLM, got %q", result.Current)
	}
}

// TestListDatabaseConnectionsTool_ToolDescription tests that tool description includes port field
func TestListDatabaseConnectionsTool_ToolDescription(t *testing.T) {
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()
	cfg := &config.Config{}

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	// Verify tool description mentions port
	if !strings.Contains(tool.Definition.Description, "port") {
		t.Error("Tool description should mention 'port' field")
	}
}

// TestSelectDatabaseConnectionTool_EmptyNameString tests empty name string
func TestSelectDatabaseConnectionTool_EmptyNameString(t *testing.T) {
	databases := createTestDatabaseConfigs()
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := SelectDatabaseConnectionTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
		"name":      "",
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for empty name")
	}

	if len(response.Content) > 0 && !strings.Contains(response.Content[0].Text, "Missing or invalid") {
		t.Errorf("Expected 'Missing or invalid' error, got: %s", response.Content[0].Text)
	}
}

// TestListDatabaseConnectionsTool_DefaultsToNil tests allow_llm_switching defaulting to true when nil
func TestListDatabaseConnectionsTool_DefaultsToNil(t *testing.T) {
	// Create database without explicit allow_llm_switching (should default to true)
	databases := []config.NamedDatabaseConfig{
		{
			Name:     "default-db",
			Host:     "localhost",
			Port:     5432,
			Database: "default",
			User:     "user1",
			// AllowLLMSwitching not set - should default to true
		},
	}
	cfg := &config.Config{Databases: databases}
	clientManager := database.NewClientManager(databases)
	defer clientManager.CloseAll()

	tool := ListDatabaseConnectionsTool(clientManager, nil, cfg)

	args := map[string]interface{}{
		"__context": context.Background(),
	}
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	var result struct {
		Databases []struct {
			Name string `json:"name"`
		} `json:"databases"`
	}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Database with nil AllowLLMSwitching should be included (defaults to true)
	if len(result.Databases) != 1 {
		t.Errorf("Expected 1 database (nil defaults to allowed), got %d", len(result.Databases))
	}
	if len(result.Databases) > 0 && result.Databases[0].Name != "default-db" {
		t.Errorf("Expected 'default-db', got %q", result.Databases[0].Name)
	}
}
