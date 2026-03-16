/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"net/url"
	"os"
	"strconv"
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

// testDBConfig parses TEST_PGEDGE_POSTGRES_CONNECTION_STRING into a
// NamedDatabaseConfig suitable for use with NewClientManagerWithConfig.
// It returns nil if the connection string cannot be parsed.
func testDBConfig(t *testing.T) *config.NamedDatabaseConfig {
	t.Helper()
	connStr := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	u, err := url.Parse(connStr)
	if err != nil {
		t.Fatalf("Failed to parse TEST_PGEDGE_POSTGRES_CONNECTION_STRING: %v", err)
	}

	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := 5432
	if u.Port() != "" {
		p, err := strconv.Atoi(u.Port())
		if err != nil {
			t.Fatalf("invalid port %q in TEST_PGEDGE_POSTGRES_CONNECTION_STRING: %v", u.Port(), err)
		}
		port = p
	}
	dbName := "postgres"
	if len(u.Path) > 1 {
		dbName = u.Path[1:]
	}
	user := ""
	password := ""
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}
	sslMode := u.Query().Get("sslmode")

	return &config.NamedDatabaseConfig{
		Name:     "testdb",
		Host:     host,
		Port:     port,
		Database: dbName,
		User:     user,
		Password: password,
		SSLMode:  sslMode,
	}
}

// TestClientManager_GetClient tests that different tokens get different clients
func TestClientManager_GetClient(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManagerWithConfig(testDBConfig(t))
	defer cm.CloseAll()

	t.Run("creates new client for new token", func(t *testing.T) {
		client1, err := cm.GetClient("token-hash-1")
		if err != nil {
			t.Fatalf("Failed to get client: %v", err)
		}
		if client1 == nil {
			t.Fatal("Expected client, got nil")
		}

		// Verify client count
		if count := cm.GetClientCount(); count != 1 {
			t.Fatalf("Expected 1 client, got %d", count)
		}
	})

	t.Run("returns same client for same token", func(t *testing.T) {
		client1, err := cm.GetClient("token-hash-2")
		if err != nil {
			t.Fatalf("Failed to get first client: %v", err)
		}

		client2, err := cm.GetClient("token-hash-2")
		if err != nil {
			t.Fatalf("Failed to get second client: %v", err)
		}

		if client1 != client2 {
			t.Fatal("Expected same client instance for same token")
		}

		// Client count should still be 2 (token-hash-1 and token-hash-2)
		if count := cm.GetClientCount(); count != 2 {
			t.Fatalf("Expected 2 clients, got %d", count)
		}
	})

	t.Run("different tokens get different clients", func(t *testing.T) {
		client1, err := cm.GetClient("token-hash-3")
		if err != nil {
			t.Fatalf("Failed to get first client: %v", err)
		}

		client2, err := cm.GetClient("token-hash-4")
		if err != nil {
			t.Fatalf("Failed to get second client: %v", err)
		}

		if client1 == client2 {
			t.Fatal("Expected different client instances for different tokens")
		}

		// Client count should now be 4
		if count := cm.GetClientCount(); count != 4 {
			t.Fatalf("Expected 4 clients, got %d", count)
		}
	})

	t.Run("rejects empty token hash", func(t *testing.T) {
		_, err := cm.GetClient("")
		if err == nil {
			t.Fatal("Expected error for empty token hash")
		}
	})
}

// TestClientManager_RemoveClient tests removing individual clients
func TestClientManager_RemoveClient(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManagerWithConfig(testDBConfig(t))
	defer cm.CloseAll()

	// Create some clients
	_, err := cm.GetClient("token-hash-a")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	_, err = cm.GetClient("token-hash-b")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if count := cm.GetClientCount(); count != 2 {
		t.Fatalf("Expected 2 clients, got %d", count)
	}

	// Remove one client
	err = cm.RemoveClient("token-hash-a")
	if err != nil {
		t.Fatalf("Failed to remove client: %v", err)
	}

	if count := cm.GetClientCount(); count != 1 {
		t.Fatalf("Expected 1 client after removal, got %d", count)
	}

	// Removing non-existent client should not error
	err = cm.RemoveClient("token-hash-nonexistent")
	if err != nil {
		t.Fatalf("Expected no error for non-existent client, got: %v", err)
	}
}

// TestClientManager_RemoveClients tests removing multiple clients at once
func TestClientManager_RemoveClients(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManagerWithConfig(testDBConfig(t))
	defer cm.CloseAll()

	// Create several clients
	for i := 1; i <= 5; i++ {
		tokenHash := "token-hash-" + string(rune('0'+i))
		_, err := cm.GetClient(tokenHash)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
	}

	if count := cm.GetClientCount(); count != 5 {
		t.Fatalf("Expected 5 clients, got %d", count)
	}

	// Remove multiple clients
	toRemove := []string{"token-hash-1", "token-hash-2", "token-hash-3"}
	err := cm.RemoveClients(toRemove)
	if err != nil {
		t.Fatalf("Failed to remove clients: %v", err)
	}

	if count := cm.GetClientCount(); count != 2 {
		t.Fatalf("Expected 2 clients after removal, got %d", count)
	}
}

// TestClientManager_CloseAll tests closing all clients
func TestClientManager_CloseAll(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManagerWithConfig(testDBConfig(t))

	// Create several clients
	for i := 1; i <= 3; i++ {
		tokenHash := "token-hash-x" + string(rune('0'+i))
		_, err := cm.GetClient(tokenHash)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
	}

	if count := cm.GetClientCount(); count != 3 {
		t.Fatalf("Expected 3 clients, got %d", count)
	}

	// Close all clients
	err := cm.CloseAll()
	if err != nil {
		t.Fatalf("Failed to close all clients: %v", err)
	}

	if count := cm.GetClientCount(); count != 0 {
		t.Fatalf("Expected 0 clients after CloseAll, got %d", count)
	}
}

// TestClientManager_SetClientForDatabase tests setting clients for specific databases
func TestClientManager_SetClientForDatabase(t *testing.T) {
	cm := NewClientManager(nil)
	defer cm.CloseAll()

	t.Run("sets client for named database", func(t *testing.T) {
		client := NewClient(nil)
		err := cm.SetClientForDatabase("default", "mydb", client)
		if err != nil {
			t.Fatalf("SetClientForDatabase returned error: %v", err)
		}
		if !cm.IsConnected("default", "mydb") {
			t.Error("Expected client to be connected after SetClientForDatabase")
		}
	})

	t.Run("rejects empty key", func(t *testing.T) {
		client := NewClient(nil)
		err := cm.SetClientForDatabase("", "mydb", client)
		if err == nil {
			t.Error("Expected error for empty key")
		}
	})

	t.Run("rejects empty database name", func(t *testing.T) {
		client := NewClient(nil)
		err := cm.SetClientForDatabase("default", "", client)
		if err == nil {
			t.Error("Expected error for empty database name")
		}
	})

	t.Run("rejects nil client", func(t *testing.T) {
		err := cm.SetClientForDatabase("default", "mydb", nil)
		if err == nil {
			t.Error("Expected error for nil client")
		}
	})

	t.Run("multiple databases under same key", func(t *testing.T) {
		client1 := NewClient(nil)
		client2 := NewClient(nil)
		if err := cm.SetClientForDatabase("tok", "db1", client1); err != nil {
			t.Fatalf("SetClientForDatabase returned error: %v", err)
		}
		if err := cm.SetClientForDatabase("tok", "db2", client2); err != nil {
			t.Fatalf("SetClientForDatabase returned error: %v", err)
		}
		if !cm.IsConnected("tok", "db1") {
			t.Error("Expected db1 to be connected")
		}
		if !cm.IsConnected("tok", "db2") {
			t.Error("Expected db2 to be connected")
		}
	})
}

// TestClientManager_IsConnected tests connection status checking
func TestClientManager_IsConnected(t *testing.T) {
	cm := NewClientManager(nil)
	defer cm.CloseAll()

	t.Run("returns false for unknown token", func(t *testing.T) {
		if cm.IsConnected("unknown-token", "some-db") {
			t.Error("Expected IsConnected to return false for unknown token")
		}
	})

	t.Run("returns false for unknown database", func(t *testing.T) {
		// Create a token entry with no clients
		cm.mu.Lock()
		cm.clients["test-token"] = make(map[string]*Client)
		cm.mu.Unlock()

		if cm.IsConnected("test-token", "nonexistent-db") {
			t.Error("Expected IsConnected to return false for unknown database")
		}
	})

	t.Run("returns true for open client", func(t *testing.T) {
		client := NewClient(nil)
		cm.mu.Lock()
		cm.clients["open-token"] = map[string]*Client{"mydb": client}
		cm.mu.Unlock()

		if !cm.IsConnected("open-token", "mydb") {
			t.Error("Expected IsConnected to return true for open client")
		}
	})

	t.Run("returns false for closed client", func(t *testing.T) {
		client := NewClient(nil)
		client.Close()
		cm.mu.Lock()
		cm.clients["closed-token"] = map[string]*Client{"mydb": client}
		cm.mu.Unlock()

		if cm.IsConnected("closed-token", "mydb") {
			t.Error("Expected IsConnected to return false for closed client")
		}
	})
}

// TestClientManager_Concurrency tests thread-safety of client manager
func TestClientManager_Concurrency(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	cm := NewClientManagerWithConfig(nil)
	defer cm.CloseAll()

	// Launch multiple goroutines trying to get the same client
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := cm.GetClient("concurrent-token")
			if err != nil {
				t.Errorf("Failed to get client: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should only have one client despite concurrent requests
	if count := cm.GetClientCount(); count != 1 {
		t.Fatalf("Expected 1 client despite concurrent access, got %d", count)
	}
}
