package feeders

import (
	"fmt"

	"github.com/golobby/config/v3/pkg/feeder"
	"gopkg.in/yaml.v3"
)

// YamlFeeder is a feeder that reads YAML files
type YamlFeeder struct {
	feeder.Yaml
}

// NewYamlFeeder creates a new YamlFeeder that reads from the specified YAML file
func NewYamlFeeder(filePath string) YamlFeeder {
	return YamlFeeder{feeder.Yaml{Path: filePath}}
}

// FeedKey reads a YAML file and extracts a specific key
func (y YamlFeeder) FeedKey(key string, target interface{}) error {
	// Create a temporary map to hold all YAML data
	var allData map[interface{}]interface{}

	// Use the embedded Yaml feeder to read the file
	if err := y.Feed(&allData); err != nil {
		return fmt.Errorf("failed to read YAML: %w", err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		return nil
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err = yaml.Unmarshal(valueBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal value to target: %w", err)
	}

	return nil
}
