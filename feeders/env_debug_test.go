package feeders

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvFeederDebugPointers(t *testing.T) {
	// Simple test structure
	type DebugConfig struct {
		DatabaseURL *string `env:"DATABASE_URL"`
	}

	// Set environment variable
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/testdb")

	// Test without debug logging to keep it simple
	feeder := NewEnvFeeder()
	config := &DebugConfig{}

	fmt.Println("=== Starting Environment Feed Debug Test ===")
	err := feeder.Feed(config)
	fmt.Printf("=== Feed Result: err=%v, config=%+v ===\n", err, config)
	if config.DatabaseURL != nil {
		fmt.Printf("DatabaseURL value: %s\n", *config.DatabaseURL)
	} else {
		fmt.Println("DatabaseURL is nil")
	}

	require.NoError(t, err)
}
