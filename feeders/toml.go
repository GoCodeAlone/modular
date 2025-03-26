package feeders

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/golobby/config/v3/pkg/feeder"
)

// TomlFeeder is a feeder that reads TOML files
type TomlFeeder struct {
	feeder.Toml
}

func NewTomlFeeder(filePath string) TomlFeeder {
	return TomlFeeder{feeder.Toml{Path: filePath}}
}

// FeedKey reads a TOML file and extracts a specific key
func (t TomlFeeder) FeedKey(key string, target interface{}) error {
	// Create a temporary map to hold all toml data
	var allData map[string]interface{}

	// Use the embedded Toml feeder to read the file
	if err := t.Feed(&allData); err != nil {
		return fmt.Errorf("failed to read toml: %w", err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		//return fmt.Errorf("key '%s' not found in toml data", key)
		return nil
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err = toml.Unmarshal(valueBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal value to target: %w", err)
	}

	return nil
}
