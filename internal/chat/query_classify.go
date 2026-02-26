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

import "strings"

// QueryType represents the type of SQL query
type QueryType int

const (
	QueryTypeSelect QueryType = iota
	QueryTypeDDL
	QueryTypeDML
	QueryTypeOther
)

// ClassifyQuery determines if a SQL query is a read or write
// operation. Returns the query type and whether the query modifies
// data.
func ClassifyQuery(sql string) (QueryType, bool) {
	upper := strings.ToUpper(strings.TrimSpace(sql))

	switch {
	case strings.HasPrefix(upper, "SELECT"),
		strings.HasPrefix(upper, "WITH"),
		strings.HasPrefix(upper, "TABLE"),
		strings.HasPrefix(upper, "VALUES"),
		strings.HasPrefix(upper, "EXPLAIN"),
		strings.HasPrefix(upper, "SHOW"):
		return QueryTypeSelect, false

	case strings.HasPrefix(upper, "CREATE"),
		strings.HasPrefix(upper, "DROP"),
		strings.HasPrefix(upper, "ALTER"),
		strings.HasPrefix(upper, "TRUNCATE"):
		return QueryTypeDDL, true

	case strings.HasPrefix(upper, "INSERT"),
		strings.HasPrefix(upper, "UPDATE"),
		strings.HasPrefix(upper, "DELETE"):
		return QueryTypeDML, true

	default:
		return QueryTypeOther, true
	}
}
