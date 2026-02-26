/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/mcp"
)

func TestBuildSpec_TopLevel(t *testing.T) {
	spec := BuildSpec()

	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi 3.0.3, got %v", spec["openapi"])
	}

	info, ok := spec["info"].(M)
	if !ok {
		t.Fatal("info is not a map")
	}
	if info["title"] != "pgEdge Postgres MCP Server API" {
		t.Errorf("unexpected title: %v", info["title"])
	}
	if info["version"] != mcp.ServerVersion {
		t.Errorf("expected version %s, got %v",
			mcp.ServerVersion, info["version"])
	}

	if _, ok := spec["paths"].(M); !ok {
		t.Error("paths should be a map")
	}
	if _, ok := spec["components"].(M); !ok {
		t.Error("components should be a map")
	}
	if _, ok := spec["tags"].(A); !ok {
		t.Error("tags should be an array")
	}
}

func TestBuildSpec_RequiredPaths(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)

	requiredPaths := []string{
		"/health",
		"/mcp/v1",
		"/api/databases",
		"/api/databases/select",
		"/api/user/info",
		"/api/chat/compact",
		"/api/llm/providers",
		"/api/llm/models",
		"/api/llm/chat",
		"/api/conversations",
		"/api/conversations/{id}",
		"/api/openapi.json",
	}

	for _, path := range requiredPaths {
		if _, ok := paths[path]; !ok {
			t.Errorf("missing required path: %s", path)
		}
	}
}

func TestBuildSpec_HealthEndpoint(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)
	health := paths["/health"].(M)

	get, ok := health["get"].(M)
	if !ok {
		t.Fatal("/health should have a GET operation")
	}

	tags := get["tags"].(A)
	if len(tags) == 0 || tags[0] != "Health" {
		t.Error("health endpoint should be tagged 'Health'")
	}

	// Health should not require auth (no security field)
	if _, hasSecurity := get["security"]; hasSecurity {
		t.Error("health endpoint should not require authentication")
	}
}

func TestBuildSpec_MCPEndpoint(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)
	mcpPath := paths["/mcp/v1"].(M)

	post, ok := mcpPath["post"].(M)
	if !ok {
		t.Fatal("/mcp/v1 should have a POST operation")
	}

	// Should require auth
	if _, hasSecurity := post["security"]; !hasSecurity {
		t.Error("/mcp/v1 should require authentication")
	}

	// Should have request body
	if _, hasBody := post["requestBody"]; !hasBody {
		t.Error("/mcp/v1 should have a request body")
	}
}

func TestBuildSpec_ConversationsEndpoint(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)
	convPath := paths["/api/conversations"].(M)

	// Should have GET, POST, DELETE
	for _, method := range []string{"get", "post", "delete"} {
		if _, ok := convPath[method].(M); !ok {
			t.Errorf("/api/conversations should have %s operation",
				strings.ToUpper(method))
		}
	}
}

func TestBuildSpec_ConversationByIDEndpoint(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)
	convIDPath := paths["/api/conversations/{id}"].(M)

	// Should have GET, PUT, PATCH, DELETE
	for _, method := range []string{"get", "put", "patch", "delete"} {
		if _, ok := convIDPath[method].(M); !ok {
			t.Errorf("/api/conversations/{id} should have %s operation",
				strings.ToUpper(method))
		}
	}

	// Should have path parameter
	params, ok := convIDPath["parameters"].(A)
	if !ok || len(params) == 0 {
		t.Error("should have path parameter for id")
	}
}

func TestBuildSpec_SecurityScheme(t *testing.T) {
	spec := BuildSpec()
	components := spec["components"].(M)
	schemes := components["securitySchemes"].(M)
	bearer := schemes["BearerAuth"].(M)

	if bearer["type"] != "http" {
		t.Errorf("expected type 'http', got %v", bearer["type"])
	}
	if bearer["scheme"] != "bearer" {
		t.Errorf("expected scheme 'bearer', got %v", bearer["scheme"])
	}
}

func TestBuildSpec_RequiredSchemas(t *testing.T) {
	spec := BuildSpec()
	components := spec["components"].(M)
	schemas := components["schemas"].(M)

	requiredSchemas := []string{
		"ErrorResponse",
		"HealthResponse",
		"JSONRPCRequest",
		"JSONRPCResponse",
		"RPCError",
		"DatabaseInfo",
		"ListDatabasesResponse",
		"SelectDatabaseRequest",
		"SelectDatabaseResponse",
		"UserInfoResponse",
		"ChatMessage",
		"CompactRequest",
		"CompactResponse",
		"ProviderInfo",
		"ProvidersResponse",
		"ModelInfo",
		"ModelsResponse",
		"LLMChatRequest",
		"LLMChatResponse",
		"ConversationSummary",
		"Conversation",
		"CreateConversationRequest",
		"RenameConversationRequest",
		"SuccessResponse",
		"DeleteAllResponse",
	}

	for _, name := range requiredSchemas {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing required schema: %s", name)
		}
	}
}

func TestBuildSpec_JSONSerializable(t *testing.T) {
	spec := BuildSpec()

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("spec should be JSON serializable: %v", err)
	}

	if len(data) == 0 {
		t.Error("serialized spec should not be empty")
	}

	// Verify round-trip
	var parsed M
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("should parse back as JSON: %v", err)
	}

	if parsed["openapi"] != "3.0.3" {
		t.Error("round-trip should preserve openapi version")
	}
}

func TestBuildSpec_LLMModelsParameters(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)
	modelsPath := paths["/api/llm/models"].(M)
	get := modelsPath["get"].(M)

	params, ok := get["parameters"].(A)
	if !ok || len(params) == 0 {
		t.Fatal("/api/llm/models should have parameters")
	}

	param := params[0].(M)
	if param["name"] != "provider" {
		t.Errorf("expected parameter 'provider', got %v", param["name"])
	}
	if param["required"] != true {
		t.Error("provider parameter should be required")
	}
}

func TestBuildSpec_DatabasesSelectResponses(t *testing.T) {
	spec := BuildSpec()
	paths := spec["paths"].(M)
	selectPath := paths["/api/databases/select"].(M)
	post := selectPath["post"].(M)
	responses := post["responses"].(M)

	// Should have 200, 400, 403, 404
	expectedCodes := []string{"200", "400", "403", "404"}
	for _, code := range expectedCodes {
		if _, ok := responses[code]; !ok {
			t.Errorf("/api/databases/select should have %s response",
				code)
		}
	}
}

func TestBuildSpec_Refs(t *testing.T) {
	// Verify all $ref values point to existing schemas
	spec := BuildSpec()
	components := spec["components"].(M)
	schemas := components["schemas"].(M)

	data, _ := json.Marshal(spec)
	specStr := string(data)

	// Find all $ref values
	prefix := "#/components/schemas/"
	idx := 0
	for {
		pos := strings.Index(specStr[idx:], prefix)
		if pos == -1 {
			break
		}
		start := idx + pos + len(prefix)
		end := start
		for end < len(specStr) && specStr[end] != '"' {
			end++
		}
		schemaName := specStr[start:end]

		if _, exists := schemas[schemaName]; !exists {
			t.Errorf("$ref points to non-existent schema: %s", schemaName)
		}
		idx = end
	}
}
