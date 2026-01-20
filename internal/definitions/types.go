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

// Definitions contains user-defined prompts, resources, and tools loaded from a file
type Definitions struct {
	Prompts   []PromptDefinition   `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	Resources []ResourceDefinition `json:"resources,omitempty" yaml:"resources,omitempty"`
	Tools     []ToolDefinition     `json:"tools,omitempty" yaml:"tools,omitempty"`
}

// PromptDefinition defines a user-defined prompt with templates
type PromptDefinition struct {
	Name        string        `json:"name" yaml:"name"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
	Arguments   []ArgumentDef `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	Messages    []MessageDef  `json:"messages" yaml:"messages"`
}

// ArgumentDef defines a prompt argument
type ArgumentDef struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required" yaml:"required"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"` // "string" (default), "boolean"
}

// MessageDef defines a message in a prompt
type MessageDef struct {
	Role    string     `json:"role" yaml:"role"`
	Content ContentDef `json:"content" yaml:"content"`
}

// ContentDef defines message content (text, image, or resource)
type ContentDef struct {
	Type     string `json:"type" yaml:"type"`
	Text     string `json:"text,omitempty" yaml:"text,omitempty"`
	Data     string `json:"data,omitempty" yaml:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty" yaml:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

// ResourceDefinition defines a user-defined resource
type ResourceDefinition struct {
	URI         string      `json:"uri" yaml:"uri"`
	Name        string      `json:"name" yaml:"name"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	MimeType    string      `json:"mimeType,omitempty" yaml:"mimeType,omitempty"`
	Type        string      `json:"type" yaml:"type"` // "sql" or "static"
	SQL         string      `json:"sql,omitempty" yaml:"sql,omitempty"`
	Data        interface{} `json:"data,omitempty" yaml:"data,omitempty"`
}

// ToolDefinition defines a user-defined custom tool
type ToolDefinition struct {
	Name        string          `json:"name" yaml:"name"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string          `json:"type" yaml:"type"` // "sql", "pl-do", or "pl-func"
	InputSchema ToolInputSchema `json:"input_schema" yaml:"input_schema"`
	SQL         string          `json:"sql,omitempty" yaml:"sql,omitempty"`           // For "sql" type
	Language    string          `json:"language,omitempty" yaml:"language,omitempty"` // For "pl-do" and "pl-func" types
	Code        string          `json:"code,omitempty" yaml:"code,omitempty"`         // For "pl-do" and "pl-func" types
	Returns     string          `json:"returns,omitempty" yaml:"returns,omitempty"`   // For "pl-func" type (e.g., "jsonb", "TABLE(id int, name text)")
	Timeout     string          `json:"timeout,omitempty" yaml:"timeout,omitempty"`   // Execution timeout (e.g., "30s", "1m")
}

// ToolInputSchema defines the input schema for a custom tool (MCP-compatible)
type ToolInputSchema struct {
	Type       string                  `json:"type" yaml:"type"` // Always "object"
	Properties map[string]ToolProperty `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required   []string                `json:"required,omitempty" yaml:"required,omitempty"`
}

// ToolProperty defines a single property in the tool input schema
type ToolProperty struct {
	Type        string        `json:"type" yaml:"type"` // "string", "integer", "number", "boolean", "array", "object"
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
	Default     interface{}   `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []string      `json:"enum,omitempty" yaml:"enum,omitempty"`
	Items       *ToolProperty `json:"items,omitempty" yaml:"items,omitempty"` // For array types
}
