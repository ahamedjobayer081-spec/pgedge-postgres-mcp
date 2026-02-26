/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import "testing"

func TestClassifyQuery(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantType  QueryType
		wantWrite bool
	}{
		// Read queries
		{"select", "SELECT * FROM users", QueryTypeSelect, false},
		{"select lowercase", "select id from orders", QueryTypeSelect, false},
		{"select mixed case", "Select Name From Products", QueryTypeSelect, false},
		{"select with leading space", "  SELECT 1", QueryTypeSelect, false},
		{"with cte", "WITH cte AS (SELECT 1) SELECT * FROM cte", QueryTypeSelect, false},
		{"table command", "TABLE users", QueryTypeSelect, false},
		{"values expression", "VALUES (1, 'a'), (2, 'b')", QueryTypeSelect, false},
		{"explain", "EXPLAIN SELECT * FROM users", QueryTypeSelect, false},
		{"explain analyze", "EXPLAIN ANALYZE SELECT * FROM users", QueryTypeSelect, false},
		{"show", "SHOW search_path", QueryTypeSelect, false},

		// DDL queries
		{"create table", "CREATE TABLE test (id int)", QueryTypeDDL, true},
		{"create index", "CREATE INDEX idx ON test (id)", QueryTypeDDL, true},
		{"drop table", "DROP TABLE IF EXISTS test", QueryTypeDDL, true},
		{"alter table", "ALTER TABLE test ADD COLUMN name text", QueryTypeDDL, true},
		{"truncate", "TRUNCATE TABLE test", QueryTypeDDL, true},
		{"create lowercase", "create table t (id int)", QueryTypeDDL, true},

		// DML queries
		{"insert", "INSERT INTO users (name) VALUES ('Alice')", QueryTypeDML, true},
		{"update", "UPDATE users SET name = 'Bob' WHERE id = 1", QueryTypeDML, true},
		{"delete", "DELETE FROM users WHERE id = 1", QueryTypeDML, true},
		{"insert lowercase", "insert into t values (1)", QueryTypeDML, true},

		// Other queries (treated as write)
		{"grant", "GRANT SELECT ON users TO reader", QueryTypeOther, true},
		{"revoke", "REVOKE ALL ON users FROM reader", QueryTypeOther, true},
		{"vacuum", "VACUUM ANALYZE users", QueryTypeOther, true},
		{"analyze", "ANALYZE users", QueryTypeOther, true},
		{"begin", "BEGIN", QueryTypeOther, true},
		{"commit", "COMMIT", QueryTypeOther, true},
		{"set", "SET timezone TO 'UTC'", QueryTypeOther, true},

		// Edge cases
		{"empty string", "", QueryTypeOther, true},
		{"whitespace only", "   ", QueryTypeOther, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotWrite := ClassifyQuery(tt.sql)
			if gotType != tt.wantType {
				t.Errorf("ClassifyQuery(%q) type = %d, want %d",
					tt.sql, gotType, tt.wantType)
			}
			if gotWrite != tt.wantWrite {
				t.Errorf("ClassifyQuery(%q) isWrite = %v, want %v",
					tt.sql, gotWrite, tt.wantWrite)
			}
		})
	}
}
