/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"context"
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

// helper to build a context with API token flag and optional token hash
func apiTokenContext(tokenHash string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, IsAPITokenContextKey, true)
	if tokenHash != "" {
		ctx = context.WithValue(ctx, TokenHashContextKey, tokenHash)
	}
	return ctx
}

// helper to build a context with a session username
func sessionUserContext(username string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, UsernameContextKey, username)
	return ctx
}

func testDatabases() []config.NamedDatabaseConfig {
	return []config.NamedDatabaseConfig{
		{Name: "db1", Database: "db1"},
		{Name: "db2", Database: "db2"},
		{Name: "db3", Database: "db3"},
	}
}

func TestGetAccessibleDatabases(t *testing.T) {
	databases := testDatabases()

	t.Run("STDIO mode returns all databases", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, true)
		result := dac.GetAccessibleDatabases(context.Background(), databases)
		if len(result) != len(databases) {
			t.Fatalf("expected %d databases, got %d", len(databases), len(result))
		}
	})

	t.Run("auth disabled returns all databases", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, false, false)
		result := dac.GetAccessibleDatabases(context.Background(), databases)
		if len(result) != len(databases) {
			t.Fatalf("expected %d databases, got %d", len(databases), len(result))
		}
	})

	t.Run("unbound API token returns all databases", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		ctx := apiTokenContext("")
		result := dac.GetAccessibleDatabases(ctx, databases)
		if len(result) != len(databases) {
			t.Fatalf("expected %d databases, got %d", len(databases), len(result))
		}
		for i, db := range result {
			if db.Name != databases[i].Name {
				t.Errorf("database %d: expected name %q, got %q", i, databases[i].Name, db.Name)
			}
		}
	})

	t.Run("bound API token returns only bound database", func(t *testing.T) {
		store := InitializeTokenStore()
		err := store.AddToken("test-token", "testhash123", "test", nil, "db2")
		if err != nil {
			t.Fatalf("failed to add token: %v", err)
		}

		dac := NewDatabaseAccessChecker(store, true, false)
		ctx := apiTokenContext("testhash123")
		result := dac.GetAccessibleDatabases(ctx, databases)
		if len(result) != 1 {
			t.Fatalf("expected 1 database, got %d", len(result))
		}
		if result[0].Name != "db2" {
			t.Errorf("expected database name %q, got %q", "db2", result[0].Name)
		}
	})

	t.Run("bound API token with nonexistent database returns nil", func(t *testing.T) {
		store := InitializeTokenStore()
		err := store.AddToken("test-token", "testhash456", "test", nil, "nonexistent")
		if err != nil {
			t.Fatalf("failed to add token: %v", err)
		}

		dac := NewDatabaseAccessChecker(store, true, false)
		ctx := apiTokenContext("testhash456")
		result := dac.GetAccessibleDatabases(ctx, databases)
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("session user with access returns filtered databases", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		dbs := []config.NamedDatabaseConfig{
			{Name: "db1", Database: "db1", AvailableToUsers: []string{"alice", "bob"}},
			{Name: "db2", Database: "db2", AvailableToUsers: []string{"charlie"}},
			{Name: "db3", Database: "db3", AvailableToUsers: []string{}},
		}
		ctx := sessionUserContext("alice")
		result := dac.GetAccessibleDatabases(ctx, dbs)
		// alice should see db1 (explicitly listed) and db3 (empty = all users)
		if len(result) != 2 {
			t.Fatalf("expected 2 databases, got %d", len(result))
		}
		if result[0].Name != "db1" || result[1].Name != "db3" {
			t.Errorf("unexpected databases: %v", result)
		}
	})

	t.Run("session user with no username returns nil", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		ctx := context.Background()
		result := dac.GetAccessibleDatabases(ctx, databases)
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("empty database list returns empty", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		ctx := apiTokenContext("")
		result := dac.GetAccessibleDatabases(ctx, nil)
		if len(result) != 0 {
			t.Fatalf("expected 0 databases, got %d", len(result))
		}
	})
}

func TestCanAccessDatabase(t *testing.T) {
	db := &config.NamedDatabaseConfig{Name: "testdb", Database: "testdb"}

	t.Run("STDIO mode allows access", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, true)
		if !dac.CanAccessDatabase(context.Background(), db) {
			t.Fatal("expected access to be allowed in STDIO mode")
		}
	})

	t.Run("auth disabled allows access", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, false, false)
		if !dac.CanAccessDatabase(context.Background(), db) {
			t.Fatal("expected access to be allowed with auth disabled")
		}
	})

	t.Run("API token allows access", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		ctx := apiTokenContext("")
		if !dac.CanAccessDatabase(ctx, db) {
			t.Fatal("expected access to be allowed for API token")
		}
	})

	t.Run("session user with access allowed", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		dbWithUsers := &config.NamedDatabaseConfig{
			Name:             "testdb",
			AvailableToUsers: []string{"alice"},
		}
		ctx := sessionUserContext("alice")
		if !dac.CanAccessDatabase(ctx, dbWithUsers) {
			t.Fatal("expected access to be allowed for alice")
		}
	})

	t.Run("session user without access denied", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		dbWithUsers := &config.NamedDatabaseConfig{
			Name:             "testdb",
			AvailableToUsers: []string{"bob"},
		}
		ctx := sessionUserContext("alice")
		if dac.CanAccessDatabase(ctx, dbWithUsers) {
			t.Fatal("expected access to be denied for alice")
		}
	})

	t.Run("no username denies access", func(t *testing.T) {
		dac := NewDatabaseAccessChecker(nil, true, false)
		if dac.CanAccessDatabase(context.Background(), db) {
			t.Fatal("expected access to be denied with no username")
		}
	})
}
