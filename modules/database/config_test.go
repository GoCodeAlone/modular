package database

import (
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestGetInstanceConfigs_ReturnsOriginalPointers tests that GetInstanceConfigs returns
// pointers to the original connection configs, not copies, so that instance-aware
// feeding can actually modify the original configuration.
func TestGetInstanceConfigs_ReturnsOriginalPointers(t *testing.T) {
	// Create a database config with connections
	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "postgres",
				DSN:    "postgres://localhost/original",
			},
			"secondary": {
				Driver: "mysql",
				DSN:    "mysql://localhost/original",
			},
		},
	}

	// Get instance configs
	instances := config.GetInstanceConfigs()

	// Verify we have the expected instances
	if len(instances) != 2 {
		t.Fatalf("Expected 2 instances, got %d", len(instances))
	}

	// Get the primary connection from instances
	primaryInstance, exists := instances["primary"]
	if !exists {
		t.Fatal("Primary instance not found")
	}

	primaryConn, ok := primaryInstance.(*ConnectionConfig)
	if !ok {
		t.Fatalf("Expected *ConnectionConfig, got %T", primaryInstance)
	}

	// Verify initial values
	if primaryConn.Driver != "postgres" {
		t.Errorf("Expected driver 'postgres', got '%s'", primaryConn.Driver)
	}
	if primaryConn.DSN != "postgres://localhost/original" {
		t.Errorf("Expected DSN 'postgres://localhost/original', got '%s'", primaryConn.DSN)
	}

	// Modify the connection through the instance
	primaryConn.Driver = "sqlite3"
	primaryConn.DSN = "sqlite3://localhost/modified"

	// Verify that the original config was modified (not a copy)
	originalPrimary := config.Connections["primary"]
	if originalPrimary.Driver != "sqlite3" {
		t.Errorf("Original config not modified: expected driver 'sqlite3', got '%s'", originalPrimary.Driver)
	}
	if originalPrimary.DSN != "sqlite3://localhost/modified" {
		t.Errorf("Original config not modified: expected DSN 'sqlite3://localhost/modified', got '%s'", originalPrimary.DSN)
	}

	// Verify that both pointers point to the same memory location
	if primaryConn != originalPrimary {
		t.Error("Instance config and original config are different objects - should be the same pointer")
	}
}

// TestInstanceAwareFeedingActuallyModifiesConfig tests that instance-aware feeding
// actually modifies the original database configuration, not just copies.
func TestInstanceAwareFeedingActuallyModifiesConfig(t *testing.T) {
	// Set environment variables for testing
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER": "sqlite3",
		"DB_PRIMARY_DSN":    "./test_primary.db",
		"DB_CACHE_DRIVER":   "redis",
		"DB_CACHE_DSN":      "redis://localhost:6379",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create a database config with initial values
	config := &Config{
		Default: "primary",
		Connections: map[string]*ConnectionConfig{
			"primary": {
				Driver: "postgres",
				DSN:    "postgres://localhost/original",
			},
			"cache": {
				Driver: "mysql",
				DSN:    "mysql://localhost/original",
			},
		},
	}

	// Create instance-aware feeder
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	feeder := modular.NewInstanceAwareEnvFeeder(instancePrefixFunc)

	// Get instances and feed them
	instances := config.GetInstanceConfigs()

	for instanceKey, instanceConfig := range instances {
		if err := feeder.FeedKey(instanceKey, instanceConfig); err != nil {
			t.Fatalf("Failed to feed instance %s: %v", instanceKey, err)
		}
	}

	// Verify that the original config was actually modified
	primaryConn := config.Connections["primary"]
	if primaryConn.Driver != "sqlite3" {
		t.Errorf("Expected primary driver to be 'sqlite3', got '%s'", primaryConn.Driver)
	}
	if primaryConn.DSN != "./test_primary.db" {
		t.Errorf("Expected primary DSN to be './test_primary.db', got '%s'", primaryConn.DSN)
	}

	cacheConn := config.Connections["cache"]
	if cacheConn.Driver != "redis" {
		t.Errorf("Expected cache driver to be 'redis', got '%s'", cacheConn.Driver)
	}
	if cacheConn.DSN != "redis://localhost:6379" {
		t.Errorf("Expected cache DSN to be 'redis://localhost:6379', got '%s'", cacheConn.DSN)
	}
}

// TestConnectionConfigPointerSemantics verifies that the configuration uses pointer semantics
// correctly and that changes to connection configs are preserved.
func TestConnectionConfigPointerSemantics(t *testing.T) {
	// Create a connection config
	originalConn := &ConnectionConfig{
		Driver: "postgres",
		DSN:    "postgres://localhost/test",
	}

	// Create config with pointer to connection
	config := &Config{
		Default: "test",
		Connections: map[string]*ConnectionConfig{
			"test": originalConn,
		},
	}

	// Get the connection from the map
	retrievedConn := config.Connections["test"]

	// Verify they are the same pointer
	if retrievedConn != originalConn {
		t.Error("Retrieved connection is not the same pointer as original")
	}

	// Modify through retrieved pointer
	retrievedConn.Driver = "sqlite3"
	retrievedConn.DSN = "sqlite3://test.db"

	// Verify original was modified
	if originalConn.Driver != "sqlite3" {
		t.Errorf("Original connection not modified: expected driver 'sqlite3', got '%s'", originalConn.Driver)
	}
	if originalConn.DSN != "sqlite3://test.db" {
		t.Errorf("Original connection not modified: expected DSN 'sqlite3://test.db', got '%s'", originalConn.DSN)
	}

	// Verify through map access
	mapConn := config.Connections["test"]
	if mapConn.Driver != "sqlite3" {
		t.Errorf("Map connection not modified: expected driver 'sqlite3', got '%s'", mapConn.Driver)
	}
}
