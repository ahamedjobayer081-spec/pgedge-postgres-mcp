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
	"fmt"
	"regexp"
)

var (
	// Valid message roles
	validRoles = map[string]bool{
		"user":      true,
		"assistant": true,
		"system":    true,
	}

	// Valid content types
	validContentTypes = map[string]bool{
		"text":     true,
		"image":    true,
		"resource": true,
	}

	// Valid resource types
	validResourceTypes = map[string]bool{
		"sql":    true,
		"static": true,
	}

	// Valid tool types
	validToolTypes = map[string]bool{
		"sql":     true,
		"pl-do":   true,
		"pl-func": true,
	}

	// Valid tool property types (JSON Schema types)
	validPropertyTypes = map[string]bool{
		"string":  true,
		"integer": true,
		"number":  true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	// Pattern to find template placeholders like {{arg_name}}
	placeholderPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

	// Pattern for valid PL language names (alphanumeric and underscore only)
	// Matches: plpgsql, plpython3u, plpythonu, plv8, plperl, plperlu, etc.
	validLanguagePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

	// Pattern for valid SQL return types
	// Matches: text, integer, jsonb, TABLE(...), SETOF type, etc.
	// Only allows safe characters: alphanumeric, underscore, parentheses, comma, space
	validReturnsPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_\s,()[\]]*$`)
)

// ValidateDefinitions validates all prompt, resource, and tool definitions
func ValidateDefinitions(defs *Definitions) error {
	// Track unique names/URIs
	promptNames := make(map[string]bool)
	resourceURIs := make(map[string]bool)
	toolNames := make(map[string]bool)

	// Validate prompts
	for i, prompt := range defs.Prompts {
		if err := validatePrompt(&prompt, promptNames); err != nil {
			return fmt.Errorf("prompt %d: %w", i, err)
		}
	}

	// Validate resources
	for i := range defs.Resources {
		if err := validateResource(&defs.Resources[i], resourceURIs); err != nil {
			return fmt.Errorf("resource %d: %w", i, err)
		}
	}

	// Validate tools
	for i := range defs.Tools {
		if err := validateTool(&defs.Tools[i], toolNames); err != nil {
			return fmt.Errorf("tool %d: %w", i, err)
		}
	}

	return nil
}

// validatePrompt validates a single prompt definition
func validatePrompt(prompt *PromptDefinition, seenNames map[string]bool) error {
	// Check required fields
	if prompt.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Check uniqueness
	if seenNames[prompt.Name] {
		return fmt.Errorf("duplicate prompt name: %s", prompt.Name)
	}
	seenNames[prompt.Name] = true

	// Check messages
	if len(prompt.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}

	// Build map of valid argument names
	argNames := make(map[string]bool)
	for _, arg := range prompt.Arguments {
		if arg.Name == "" {
			return fmt.Errorf("argument name is required")
		}
		argNames[arg.Name] = true
	}

	// Validate each message
	for j, msg := range prompt.Messages {
		if err := validateMessage(&msg, argNames); err != nil {
			return fmt.Errorf("message %d: %w", j, err)
		}
	}

	return nil
}

// validateMessage validates a prompt message
func validateMessage(msg *MessageDef, validArgs map[string]bool) error {
	// Validate role
	if !validRoles[msg.Role] {
		return fmt.Errorf("invalid role %q (must be user, assistant, or system)", msg.Role)
	}

	// Validate content type
	if !validContentTypes[msg.Content.Type] {
		return fmt.Errorf("invalid content type %q (must be text, image, or resource)", msg.Content.Type)
	}

	// Type-specific validation
	switch msg.Content.Type {
	case "text":
		if msg.Content.Text == "" {
			return fmt.Errorf("text content requires 'text' field")
		}
		// Check that template placeholders reference valid arguments
		matches := placeholderPattern.FindAllStringSubmatch(msg.Content.Text, -1)
		for _, match := range matches {
			argName := match[1]
			if !validArgs[argName] {
				return fmt.Errorf("template references undefined argument: %s", argName)
			}
		}
	case "image":
		if msg.Content.Data == "" {
			return fmt.Errorf("image content requires 'data' field")
		}
		if msg.Content.MimeType == "" {
			return fmt.Errorf("image content requires 'mimeType' field")
		}
	case "resource":
		if msg.Content.URI == "" {
			return fmt.Errorf("resource content requires 'uri' field")
		}
	}

	return nil
}

// validateResource validates a single resource definition
func validateResource(res *ResourceDefinition, seenURIs map[string]bool) error {
	// Check required fields
	if res.URI == "" {
		return fmt.Errorf("uri is required")
	}
	if res.Name == "" {
		return fmt.Errorf("name is required")
	}
	if res.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Check uniqueness
	if seenURIs[res.URI] {
		return fmt.Errorf("duplicate resource URI: %s", res.URI)
	}
	seenURIs[res.URI] = true

	// Validate type
	if !validResourceTypes[res.Type] {
		return fmt.Errorf("invalid type %q (must be sql or static)", res.Type)
	}

	// Set default mime type if not specified
	if res.MimeType == "" {
		res.MimeType = "application/json"
	}

	// Type-specific validation
	switch res.Type {
	case "sql":
		if res.SQL == "" {
			return fmt.Errorf("sql type requires 'sql' field")
		}
		// Note: We could warn about potentially destructive queries (INSERT, UPDATE, DELETE, etc.)
		// but that's left to the user's discretion
	case "static":
		if res.Data == nil {
			return fmt.Errorf("static type requires 'data' field")
		}
	}

	return nil
}

// GetTemplatePlaceholders extracts all {{placeholder}} names from a template string
func GetTemplatePlaceholders(template string) []string {
	matches := placeholderPattern.FindAllStringSubmatch(template, -1)
	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		placeholders = append(placeholders, match[1])
	}
	return placeholders
}

// validateTool validates a single tool definition
func validateTool(tool *ToolDefinition, seenNames map[string]bool) error {
	if err := validateToolNameAndType(tool, seenNames); err != nil {
		return err
	}

	if err := validateToolInputSchema(&tool.InputSchema); err != nil {
		return fmt.Errorf("input_schema: %w", err)
	}

	return validateToolTypeSpecific(tool)
}

// validateToolNameAndType validates tool name uniqueness and type
func validateToolNameAndType(tool *ToolDefinition, seenNames map[string]bool) error {
	if tool.Name == "" {
		return fmt.Errorf("name is required")
	}
	if seenNames[tool.Name] {
		return fmt.Errorf("duplicate tool name: %s", tool.Name)
	}
	seenNames[tool.Name] = true

	if tool.Type == "" {
		return fmt.Errorf("type is required")
	}
	if !validToolTypes[tool.Type] {
		return fmt.Errorf("invalid type %q (must be sql, pl-do, or pl-func)", tool.Type)
	}
	return nil
}

// validateToolTypeSpecific validates type-specific fields
func validateToolTypeSpecific(tool *ToolDefinition) error {
	switch tool.Type {
	case "sql":
		return validateSQLTool(tool)
	case "pl-do":
		return validatePLDOTool(tool)
	case "pl-func":
		return validatePLFuncTool(tool)
	}
	return nil
}

func validateSQLTool(tool *ToolDefinition) error {
	if tool.SQL == "" {
		return fmt.Errorf("sql type requires 'sql' field")
	}
	return nil
}

func validatePLDOTool(tool *ToolDefinition) error {
	return validatePLLanguageAndCode(tool, "pl-do")
}

func validatePLFuncTool(tool *ToolDefinition) error {
	if err := validatePLLanguageAndCode(tool, "pl-func"); err != nil {
		return err
	}
	if tool.Returns == "" {
		return fmt.Errorf("pl-func type requires 'returns' field")
	}
	if !validReturnsPattern.MatchString(tool.Returns) {
		return fmt.Errorf("invalid returns %q: must be a valid SQL type (e.g., text, jsonb, TABLE(...))", tool.Returns)
	}
	return nil
}

// validatePLLanguageAndCode validates language and code fields for PL tools
func validatePLLanguageAndCode(tool *ToolDefinition, toolType string) error {
	if tool.Language == "" {
		return fmt.Errorf("%s type requires 'language' field", toolType)
	}
	if !validLanguagePattern.MatchString(tool.Language) {
		return fmt.Errorf("invalid language %q: must be alphanumeric (e.g., plpgsql, plpython3u)", tool.Language)
	}
	if tool.Code == "" {
		return fmt.Errorf("%s type requires 'code' field", toolType)
	}
	return nil
}

// validateToolInputSchema validates a tool's input schema
func validateToolInputSchema(schema *ToolInputSchema) error {
	// Type must be "object" for MCP tools
	if schema.Type != "" && schema.Type != "object" {
		return fmt.Errorf("type must be 'object', got %q", schema.Type)
	}

	// Validate each property
	for name, prop := range schema.Properties {
		if err := validateToolProperty(name, &prop); err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
	}

	// Check that required properties exist
	for _, reqName := range schema.Required {
		if _, exists := schema.Properties[reqName]; !exists {
			return fmt.Errorf("required property %q not found in properties", reqName)
		}
	}

	return nil
}

// validateToolProperty validates a single tool property
func validateToolProperty(name string, prop *ToolProperty) error {
	if prop.Type == "" {
		return fmt.Errorf("type is required")
	}
	if !validPropertyTypes[prop.Type] {
		return fmt.Errorf("invalid type %q (must be string, integer, number, boolean, array, or object)", prop.Type)
	}

	// For array types, validate items schema if present
	if prop.Type == "array" && prop.Items != nil {
		if err := validateToolProperty("items", prop.Items); err != nil {
			return fmt.Errorf("items: %w", err)
		}
	}

	return nil
}
