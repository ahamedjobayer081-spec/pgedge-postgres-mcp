/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package definitions

import (
	"strings"
	"testing"
)

func TestValidateDefinitions_ValidPrompt(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name:        "test-prompt",
				Description: "Test",
				Arguments: []ArgumentDef{
					{Name: "arg1", Required: true},
				},
				Messages: []MessageDef{
					{
						Role: "user",
						Content: ContentDef{
							Type: "text",
							Text: "Test {{arg1}}",
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid prompt to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_PromptMissingName(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing prompt name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

func TestValidateDefinitions_PromptNoMessages(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{Name: "test", Messages: []MessageDef{}},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for prompt with no messages")
	}
}

func TestValidateDefinitions_DuplicatePromptName(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "A"}},
				},
			},
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "B"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for duplicate prompt name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected 'duplicate' error, got: %v", err)
	}
}

func TestValidateDefinitions_InvalidRole(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "invalid", Content: ContentDef{Type: "text", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid role")
	}
}

func TestValidateDefinitions_ValidRoles(t *testing.T) {
	roles := []string{"user", "assistant", "system"}
	for _, role := range roles {
		defs := &Definitions{
			Prompts: []PromptDefinition{
				{
					Name: "test",
					Messages: []MessageDef{
						{Role: role, Content: ContentDef{Type: "text", Text: "Test"}},
					},
				},
			},
		}

		err := ValidateDefinitions(defs)
		if err != nil {
			t.Errorf("Expected role '%s' to be valid, got error: %v", role, err)
		}
	}
}

func TestValidateDefinitions_InvalidContentType(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "invalid", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid content type")
	}
}

func TestValidateDefinitions_TextContentMissingText(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for text content without text field")
	}
}

func TestValidateDefinitions_UndefinedArgument(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Arguments: []ArgumentDef{
					{Name: "arg1"},
				},
				Messages: []MessageDef{
					{
						Role: "user",
						Content: ContentDef{
							Type: "text",
							Text: "Test {{undefined_arg}}",
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for undefined argument in template")
	}
	if !strings.Contains(err.Error(), "undefined argument") {
		t.Errorf("Expected 'undefined argument' error, got: %v", err)
	}
}

func TestValidateDefinitions_ValidSQLResource(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{
				URI:  "custom://test",
				Name: "Test",
				Type: "sql",
				SQL:  "SELECT 1",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid SQL resource to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ValidStaticResource(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{
				URI:  "custom://test",
				Name: "Test",
				Type: "static",
				Data: "test value",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid static resource to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ResourceMissingURI(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{Name: "Test", Type: "static", Data: "test"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing resource URI")
	}
}

func TestValidateDefinitions_ResourceMissingName(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Type: "static", Data: "test"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing resource name")
	}
}

func TestValidateDefinitions_ResourceMissingType(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing resource type")
	}
}

func TestValidateDefinitions_DuplicateResourceURI(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test1", Type: "static", Data: "a"},
			{URI: "custom://test", Name: "Test2", Type: "static", Data: "b"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for duplicate resource URI")
	}
}

func TestValidateDefinitions_InvalidResourceType(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test", Type: "invalid"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid resource type")
	}
}

func TestValidateDefinitions_SQLResourceMissingSQL(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test", Type: "sql"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for SQL resource without sql field")
	}
}

func TestValidateDefinitions_StaticResourceMissingData(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{URI: "custom://test", Name: "Test", Type: "static"},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for static resource without data field")
	}
}

func TestValidateDefinitions_DefaultMimeType(t *testing.T) {
	defs := &Definitions{
		Resources: []ResourceDefinition{
			{
				URI:  "custom://test",
				Name: "Test",
				Type: "static",
				Data: "test",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	if defs.Resources[0].MimeType != "application/json" {
		t.Errorf("Expected default mimeType 'application/json', got '%s'", defs.Resources[0].MimeType)
	}
}

func TestValidateDefinitions_ArgumentMissingName(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{
				Name: "test",
				Arguments: []ArgumentDef{
					{Description: "No name"},
				},
				Messages: []MessageDef{
					{Role: "user", Content: ContentDef{Type: "text", Text: "Test"}},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for argument without name")
	}
}

func TestGetTemplatePlaceholders(t *testing.T) {
	tests := []struct {
		template    string
		expected    []string
		description string
	}{
		{
			template:    "No placeholders",
			expected:    []string{},
			description: "Text without placeholders",
		},
		{
			template:    "Hello {{name}}",
			expected:    []string{"name"},
			description: "Single placeholder",
		},
		{
			template:    "{{greeting}} {{name}}!",
			expected:    []string{"greeting", "name"},
			description: "Multiple placeholders",
		},
		{
			template:    "{{arg1}} and {{arg1}} again",
			expected:    []string{"arg1", "arg1"},
			description: "Duplicate placeholders",
		},
		{
			template:    "Nested {{outer_{{inner}}}}",
			expected:    []string{"inner"},
			description: "Nested braces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := GetTemplatePlaceholders(tt.template)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d placeholders, got %d", len(tt.expected), len(result))
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("Expected placeholder '%s' at index %d, got '%s'", exp, i, result[i])
				}
			}
		})
	}
}

func TestValidateDefinitions_EmptyDefinitions(t *testing.T) {
	defs := &Definitions{}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected empty definitions to be valid, got error: %v", err)
	}
}

func TestValidateDefinitions_MultipleErrors(t *testing.T) {
	defs := &Definitions{
		Prompts: []PromptDefinition{
			{Name: "test1"}, // Missing messages
			{Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: "A"}}}}, // Missing name
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for multiple validation failures")
	}
	// Should report first error encountered
}

// Tool validation tests

func TestValidateDefinitions_ValidSQLTool(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:        "test_sql_tool",
				Description: "A test SQL tool",
				Type:        "sql",
				SQL:         "SELECT * FROM users WHERE id = $1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"user_id": {
							Type:        "integer",
							Description: "User ID",
						},
					},
					Required: []string{"user_id"},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid SQL tool to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ValidPLDOTool(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:        "test_pldo_tool",
				Description: "A test PL-DO tool",
				Type:        "pl-do",
				Language:    "plpgsql",
				Code:        "PERFORM set_config('mcp.tool_result', 'test', true);",
				InputSchema: ToolInputSchema{
					Type:       "object",
					Properties: map[string]ToolProperty{},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid PL-DO tool to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ValidPLFuncTool(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:        "test_plfunc_tool",
				Description: "A test PL-FUNC tool",
				Type:        "pl-func",
				Language:    "plpgsql",
				Returns:     "jsonb",
				Code:        "BEGIN RETURN '{}'::jsonb; END;",
				InputSchema: ToolInputSchema{
					Type:       "object",
					Properties: map[string]ToolProperty{},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected valid PL-FUNC tool to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ToolMissingName(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing tool name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolMissingType(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for missing tool type")
	}
	if !strings.Contains(err.Error(), "type is required") {
		t.Errorf("Expected 'type is required' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolInvalidType(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "invalid",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid tool type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("Expected 'invalid type' error, got: %v", err)
	}
}

func TestValidateDefinitions_DuplicateToolName(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 2",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for duplicate tool name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected 'duplicate' error, got: %v", err)
	}
}

func TestValidateDefinitions_SQLToolMissingSQL(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for SQL tool without sql field")
	}
	if !strings.Contains(err.Error(), "sql type requires 'sql' field") {
		t.Errorf("Expected 'sql type requires' error, got: %v", err)
	}
}

func TestValidateDefinitions_PLDOToolMissingLanguage(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "pl-do",
				Code: "NULL;",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for PL-DO tool without language")
	}
	if !strings.Contains(err.Error(), "pl-do type requires 'language' field") {
		t.Errorf("Expected 'language' field error, got: %v", err)
	}
}

func TestValidateDefinitions_PLDOToolMissingCode(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:     "test_tool",
				Type:     "pl-do",
				Language: "plpgsql",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for PL-DO tool without code")
	}
	if !strings.Contains(err.Error(), "pl-do type requires 'code' field") {
		t.Errorf("Expected 'code' field error, got: %v", err)
	}
}

func TestValidateDefinitions_PLFuncToolMissingReturns(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:     "test_tool",
				Type:     "pl-func",
				Language: "plpgsql",
				Code:     "BEGIN RETURN 1; END;",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for PL-FUNC tool without returns")
	}
	if !strings.Contains(err.Error(), "pl-func type requires 'returns' field") {
		t.Errorf("Expected 'returns' field error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolInputSchemaInvalidType(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "array", // Must be "object"
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid input schema type")
	}
	if !strings.Contains(err.Error(), "type must be 'object'") {
		t.Errorf("Expected 'type must be object' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolPropertyInvalidType(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"bad_prop": {
							Type: "invalid_type",
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid property type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("Expected 'invalid type' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolPropertyMissingType(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"bad_prop": {
							Description: "Missing type",
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for property without type")
	}
	if !strings.Contains(err.Error(), "type is required") {
		t.Errorf("Expected 'type is required' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolRequiredPropertyNotDefined(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"existing_prop": {
							Type: "string",
						},
					},
					Required: []string{"missing_prop"},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for required property not in properties")
	}
	if !strings.Contains(err.Error(), "required property") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'required property not found' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolArrayPropertyWithItems(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"tags": {
							Type: "array",
							Items: &ToolProperty{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected array property with valid items to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ToolArrayPropertyWithInvalidItems(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"tags": {
							Type: "array",
							Items: &ToolProperty{
								Type: "invalid",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for array property with invalid items type")
	}
	if !strings.Contains(err.Error(), "items") {
		t.Errorf("Expected 'items' error, got: %v", err)
	}
}

func TestValidateDefinitions_ToolWithAllPropertyTypes(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"str_prop":  {Type: "string"},
						"int_prop":  {Type: "integer"},
						"num_prop":  {Type: "number"},
						"bool_prop": {Type: "boolean"},
						"arr_prop": {
							Type:  "array",
							Items: &ToolProperty{Type: "string"},
						},
						"obj_prop": {Type: "object"},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected all valid property types to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ToolWithEmptyInputSchema(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					// Type defaults to empty, but should pass
					Properties: map[string]ToolProperty{},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected tool with empty input schema to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ToolWithTimeout(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:    "test_tool",
				Type:    "sql",
				SQL:     "SELECT 1",
				Timeout: "30s",
				InputSchema: ToolInputSchema{
					Type: "object",
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected tool with timeout to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ToolPropertyWithEnum(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"status": {
							Type: "string",
							Enum: []string{"active", "inactive", "pending"},
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected property with enum to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_ToolPropertyWithDefault(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name: "test_tool",
				Type: "sql",
				SQL:  "SELECT 1",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]ToolProperty{
						"limit": {
							Type:    "integer",
							Default: 10,
						},
					},
				},
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err != nil {
		t.Errorf("Expected property with default to pass, got error: %v", err)
	}
}

func TestValidateDefinitions_PLDOToolInvalidLanguage(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:     "test_tool",
				Type:     "pl-do",
				Language: "plpgsql; DROP TABLE users;--", // SQL injection attempt
				Code:     "BEGIN NULL; END;",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid language name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid language") {
		t.Errorf("Expected 'invalid language' error, got: %v", err)
	}
}

func TestValidateDefinitions_PLFuncToolInvalidLanguage(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:     "test_tool",
				Type:     "pl-func",
				Language: "plpgsql' OR '1'='1",
				Code:     "BEGIN RETURN 1; END;",
				Returns:  "integer",
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid language name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid language") {
		t.Errorf("Expected 'invalid language' error, got: %v", err)
	}
}

func TestValidateDefinitions_PLFuncToolInvalidReturns(t *testing.T) {
	defs := &Definitions{
		Tools: []ToolDefinition{
			{
				Name:     "test_tool",
				Type:     "pl-func",
				Language: "plpgsql",
				Code:     "BEGIN RETURN 1; END;",
				Returns:  "integer; DROP TABLE users;--", // SQL injection attempt
			},
		},
	}

	err := ValidateDefinitions(defs)
	if err == nil {
		t.Error("Expected error for invalid returns type, got nil")
	}
	if !strings.Contains(err.Error(), "invalid returns") {
		t.Errorf("Expected 'invalid returns' error, got: %v", err)
	}
}

func TestValidateDefinitions_PLFuncToolValidComplexReturns(t *testing.T) {
	testCases := []struct {
		name    string
		returns string
	}{
		{"simple type", "text"},
		{"array type", "integer[]"},
		{"table type", "TABLE(id integer, name text)"},
		{"setof type", "SETOF integer"},
		{"jsonb", "jsonb"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defs := &Definitions{
				Tools: []ToolDefinition{
					{
						Name:     "test_tool",
						Type:     "pl-func",
						Language: "plpgsql",
						Code:     "BEGIN RETURN NULL; END;",
						Returns:  tc.returns,
					},
				},
			}

			err := ValidateDefinitions(defs)
			if err != nil {
				t.Errorf("Expected valid returns %q to pass, got error: %v", tc.returns, err)
			}
		})
	}
}

func TestValidateDefinitions_PLToolValidLanguages(t *testing.T) {
	validLanguages := []string{
		"plpgsql",
		"plpython3u",
		"plpythonu",
		"plv8",
		"plperl",
		"plperlu",
		"pltcl",
		"plr",
	}

	for _, lang := range validLanguages {
		t.Run(lang, func(t *testing.T) {
			defs := &Definitions{
				Tools: []ToolDefinition{
					{
						Name:     "test_tool",
						Type:     "pl-do",
						Language: lang,
						Code:     "BEGIN NULL; END;",
					},
				},
			}

			err := ValidateDefinitions(defs)
			if err != nil {
				t.Errorf("Expected valid language %q to pass, got error: %v", lang, err)
			}
		})
	}
}
