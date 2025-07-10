package feeders

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Simple debug logger for testing
type DebugLogger struct{}

func (d *DebugLogger) Debug(msg string, args ...any) {
	fmt.Printf("DEBUG: %s", msg)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			fmt.Printf(" %v=%v", args[i], args[i+1])
		}
	}
	fmt.Println()
}

func TestYamlFeederDebugMapPointers(t *testing.T) {
	// Simple test structure with pointer map
	type DebugConnection struct {
		Driver string `yaml:"driver"`
		DSN    string `yaml:"dsn"`
	}

	type DebugConfig struct {
		Connections map[string]*DebugConnection `yaml:"connections"`
	}

	yamlContent := `
connections:
  test:
    driver: "postgres"
    dsn: "postgres://localhost/test"
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "debug-yaml-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with debug logging
	feeder := NewYamlFeeder(tmpFile.Name())
	logger := &DebugLogger{}
	feeder.SetVerboseDebug(true, logger)

	config := &DebugConfig{}

	fmt.Println("=== Starting YAML Feed Debug Test ===")
	err = feeder.Feed(config)
	fmt.Printf("=== Feed Result: err=%v, connections=%+v ===\n", err, config.Connections)

	require.NoError(t, err)
}
