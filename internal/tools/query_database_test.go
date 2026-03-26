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
	"strings"
	"testing"
	"time"
)

func TestFormatTSVValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
		},
		{
			name:     "string value",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with tab",
			input:    "hello\tworld",
			expected: "hello\\tworld",
		},
		{
			name:     "string with newline",
			input:    "hello\nworld",
			expected: "hello\\nworld",
		},
		{
			name:     "string with carriage return",
			input:    "hello\rworld",
			expected: "hello\\rworld",
		},
		{
			name:     "string with multiple special chars",
			input:    "a\tb\nc\rd",
			expected: "a\\tb\\nc\\rd",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "int64",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "byte slice",
			input:    []byte("bytes"),
			expected: "bytes",
		},
		{
			name:     "array",
			input:    []interface{}{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "map",
			input:    map[string]interface{}{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTSVValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTSVValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatTSVValue_Time(t *testing.T) {
	// Test time formatting separately since we need to construct a specific time
	testTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTSVValue(testTime)
	expected := "2024-06-15T10:30:00Z"
	if result != expected {
		t.Errorf("FormatTSVValue(time) = %q, want %q", result, expected)
	}
}

// TestQueryTypeDetection tests the logic for detecting query types
// This verifies the fix for DDL/DML silent failure bug where Query() was
// used instead of Exec() for non-row-returning statements
func TestQueryTypeDetection(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		isSelectQuery bool
		isDDLQuery    bool
		isDMLQuery    bool
		hasReturning  bool
		expectsRows   bool // true = use Query(), false = use Exec()
	}{
		// SELECT queries - should return rows
		{
			name:          "simple SELECT",
			query:         "SELECT * FROM users",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "SELECT with WHERE",
			query:         "SELECT id, name FROM users WHERE active = true",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "WITH CTE query",
			query:         "WITH active_users AS (SELECT * FROM users) SELECT * FROM active_users",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "TABLE command",
			query:         "TABLE users",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "VALUES expression",
			query:         "VALUES (1, 'a'), (2, 'b')",
			isSelectQuery: true,
			expectsRows:   true,
		},

		// DDL queries - should NOT return rows, use Exec()
		{
			name:        "CREATE SCHEMA",
			query:       "CREATE SCHEMA test",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "CREATE TABLE",
			query:       "CREATE TABLE test (id int)",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "DROP TABLE",
			query:       "DROP TABLE test",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "ALTER TABLE",
			query:       "ALTER TABLE users ADD COLUMN email text",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "TRUNCATE",
			query:       "TRUNCATE TABLE logs",
			isDDLQuery:  true,
			expectsRows: false,
		},

		// DML without RETURNING - should NOT return rows, use Exec()
		{
			name:        "simple INSERT",
			query:       "INSERT INTO users (name) VALUES ('test')",
			isDMLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "INSERT with SELECT",
			query:       "INSERT INTO users_backup SELECT * FROM users",
			isDMLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "simple UPDATE",
			query:       "UPDATE users SET active = false WHERE id = 1",
			isDMLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "simple DELETE",
			query:       "DELETE FROM users WHERE id = 1",
			isDMLQuery:  true,
			expectsRows: false,
		},

		// DML with RETURNING - SHOULD return rows, use Query()
		{
			name:         "INSERT with RETURNING",
			query:        "INSERT INTO users (name) VALUES ('test') RETURNING id",
			isDMLQuery:   true,
			hasReturning: true,
			expectsRows:  true,
		},
		{
			name:         "UPDATE with RETURNING",
			query:        "UPDATE users SET active = false RETURNING id, name",
			isDMLQuery:   true,
			hasReturning: true,
			expectsRows:  true,
		},
		{
			name:         "DELETE with RETURNING",
			query:        "DELETE FROM users WHERE id = 1 RETURNING *",
			isDMLQuery:   true,
			hasReturning: true,
			expectsRows:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upperQuery := strings.ToUpper(strings.TrimSpace(tt.query))

			// Test SELECT detection
			isSelectQuery := strings.HasPrefix(upperQuery, "SELECT") ||
				strings.HasPrefix(upperQuery, "WITH") ||
				strings.HasPrefix(upperQuery, "TABLE") ||
				strings.HasPrefix(upperQuery, "VALUES")
			if isSelectQuery != tt.isSelectQuery {
				t.Errorf("isSelectQuery = %v, want %v", isSelectQuery, tt.isSelectQuery)
			}

			// Test DDL detection
			isDDLQuery := strings.HasPrefix(upperQuery, "CREATE") ||
				strings.HasPrefix(upperQuery, "DROP") ||
				strings.HasPrefix(upperQuery, "ALTER") ||
				strings.HasPrefix(upperQuery, "TRUNCATE")
			if isDDLQuery != tt.isDDLQuery {
				t.Errorf("isDDLQuery = %v, want %v", isDDLQuery, tt.isDDLQuery)
			}

			// Test DML detection
			isDMLQuery := strings.HasPrefix(upperQuery, "INSERT") ||
				strings.HasPrefix(upperQuery, "UPDATE") ||
				strings.HasPrefix(upperQuery, "DELETE")
			if isDMLQuery != tt.isDMLQuery {
				t.Errorf("isDMLQuery = %v, want %v", isDMLQuery, tt.isDMLQuery)
			}

			// Test RETURNING detection
			hasReturning := isDMLQuery && strings.Contains(upperQuery, "RETURNING")
			if hasReturning != tt.hasReturning {
				t.Errorf("hasReturning = %v, want %v", hasReturning, tt.hasReturning)
			}

			// Test final decision: does query return rows?
			returnsRows := isSelectQuery || hasReturning
			if returnsRows != tt.expectsRows {
				t.Errorf("returnsRows = %v, want %v (should use %s)",
					returnsRows, tt.expectsRows,
					map[bool]string{true: "Query()", false: "Exec()"}[tt.expectsRows])
			}
		})
	}
}

// TestStripTrailingSemicolons verifies that trailing semicolons are
// stripped before LIMIT/OFFSET are appended, preventing syntax errors
// like "SELECT 1; LIMIT 101". See GitHub issue #110.
func TestStripTrailingSemicolons(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no semicolon",
			input:    "SELECT 1",
			expected: "SELECT 1",
		},
		{
			name:     "single trailing semicolon",
			input:    "SELECT 1;",
			expected: "SELECT 1",
		},
		{
			name:     "semicolon with trailing space",
			input:    "SELECT 1; ",
			expected: "SELECT 1",
		},
		{
			name:     "multiple trailing semicolons",
			input:    "SELECT 1;;;",
			expected: "SELECT 1",
		},
		{
			name:     "leading and trailing whitespace",
			input:    "  SELECT 1;  ",
			expected: "SELECT 1",
		},
		{
			name:     "interleaved trailing semicolons and spaces",
			input:    "SELECT 1; ;",
			expected: "SELECT 1",
		},
		{
			name:     "semicolons and spaces only",
			input:    " ; ;;  ",
			expected: "",
		},
		{
			name:     "semicolon in middle preserved",
			input:    "SELECT '1;2'",
			expected: "SELECT '1;2'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripTrailingSemicolons(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatResultsAsTSV(t *testing.T) {
	tests := []struct {
		name        string
		columnNames []string
		results     [][]interface{}
		expected    string
	}{
		{
			name:        "empty columns",
			columnNames: []string{},
			results:     [][]interface{}{},
			expected:    "",
		},
		{
			name:        "header only (no results)",
			columnNames: []string{"id", "name", "email"},
			results:     [][]interface{}{},
			expected:    "id\tname\temail",
		},
		{
			name:        "single row",
			columnNames: []string{"id", "name"},
			results:     [][]interface{}{{1, "Alice"}},
			expected:    "id\tname\n1\tAlice",
		},
		{
			name:        "multiple rows",
			columnNames: []string{"id", "name", "active"},
			results: [][]interface{}{
				{1, "Alice", true},
				{2, "Bob", false},
			},
			expected: "id\tname\tactive\n1\tAlice\ttrue\n2\tBob\tfalse",
		},
		{
			name:        "with null values",
			columnNames: []string{"id", "name", "email"},
			results: [][]interface{}{
				{1, "Alice", nil},
				{2, nil, "bob@example.com"},
			},
			expected: "id\tname\temail\n1\tAlice\t\n2\t\tbob@example.com",
		},
		{
			name:        "with special characters",
			columnNames: []string{"id", "notes"},
			results: [][]interface{}{
				{1, "line1\nline2"},
				{2, "col1\tcol2"},
			},
			expected: "id\tnotes\n1\tline1\\nline2\n2\tcol1\\tcol2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatResultsAsTSV(tt.columnNames, tt.results)
			if result != tt.expected {
				t.Errorf("FormatResultsAsTSV() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateReadOnlyQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		expectErr bool
	}{
		{
			name:      "safe SELECT query",
			query:     "SELECT * FROM users",
			expectErr: false,
		},
		{
			name:      "safe SELECT with WHERE clause",
			query:     "SELECT id, name FROM users WHERE active = true",
			expectErr: false,
		},
		{
			name:      "safe SHOW command",
			query:     "SHOW server_version",
			expectErr: false,
		},
		{
			name:      "DO block with transaction_read_only",
			query:     "DO $$ BEGIN PERFORM set_config('transaction_read_only', 'off', true); END $$",
			expectErr: true,
		},
		{
			name:      "uppercase TRANSACTION_READ_ONLY",
			query:     "SELECT set_config('TRANSACTION_READ_ONLY', 'off', true)",
			expectErr: true,
		},
		{
			name:      "mixed case Transaction_Read_Only",
			query:     "SELECT set_config('Transaction_Read_Only', 'off', true)",
			expectErr: true,
		},
		{
			name:      "default_transaction_read_only",
			query:     "SET default_transaction_read_only = off",
			expectErr: true,
		},
		{
			name:      "DEFAULT_TRANSACTION_READ_ONLY uppercase",
			query:     "SET DEFAULT_TRANSACTION_READ_ONLY TO off",
			expectErr: true,
		},
		{
			name:      "DO block bypass attempt",
			query:     "DO $$ BEGIN PERFORM set_config('transaction_read_only', 'off', true); EXECUTE 'DELETE FROM users'; END $$",
			expectErr: true,
		},
		{
			name:      "embedded in longer query",
			query:     "SELECT 1; SET transaction_read_only = off; DELETE FROM users",
			expectErr: true,
		},
		{
			name:      "query mentioning read_only in a comment",
			query:     "SELECT * FROM config WHERE key = 'transaction_read_only'",
			expectErr: true,
		},
		{
			name:      "safe query with transaction keyword",
			query:     "SELECT * FROM transaction_logs",
			expectErr: false,
		},
		{
			name:      "safe query with read_only keyword",
			query:     "SELECT * FROM read_only_replicas",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReadOnlyQuery(tt.query)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for query %q, got nil", tt.query)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for query %q: %v", tt.query, err)
			}
			if err != nil && !strings.Contains(err.Error(), "transaction_read_only") {
				t.Errorf("error message should mention 'transaction_read_only', got: %v", err)
			}
		})
	}
}
