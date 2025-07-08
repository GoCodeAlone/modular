package feeders

import (
	"fmt"
	"reflect"

	"github.com/golobby/config/v3/pkg/feeder"
	"gopkg.in/yaml.v3"
)

// YamlFeeder is a feeder that reads YAML files with optional verbose debug logging
type YamlFeeder struct {
	feeder.Yaml
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
}

// NewYamlFeeder creates a new YamlFeeder that reads from the specified YAML file
func NewYamlFeeder(filePath string) YamlFeeder {
	return YamlFeeder{
		Yaml:         feeder.Yaml{Path: filePath},
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (y *YamlFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	y.verboseDebug = enabled
	y.logger = logger
	if enabled && logger != nil {
		y.logger.Debug("Verbose YAML feeder debugging enabled")
	}
}

// Feed reads the YAML file and populates the provided structure
func (y YamlFeeder) Feed(structure interface{}) error {
	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Starting feed process", "filePath", y.Path, "structureType", reflect.TypeOf(structure))
	}

	err := y.Yaml.Feed(structure)
	if y.verboseDebug && y.logger != nil {
		if err != nil {
			y.logger.Debug("YamlFeeder: Feed completed with error", "filePath", y.Path, "error", err)
		} else {
			y.logger.Debug("YamlFeeder: Feed completed successfully", "filePath", y.Path)
		}
	}
	if err != nil {
		return fmt.Errorf("yaml feed error: %w", err)
	}
	return nil
}

// FeedKey reads a YAML file and extracts a specific key
func (y YamlFeeder) FeedKey(key string, target interface{}) error {
	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Starting FeedKey process", "filePath", y.Path, "key", key, "targetType", reflect.TypeOf(target))
	}

	// Create a temporary map to hold all YAML data
	var allData map[interface{}]interface{}

	// Use the embedded Yaml feeder to read the file
	if err := y.Feed(&allData); err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to read YAML file", "filePath", y.Path, "error", err)
		}
		return fmt.Errorf("failed to read YAML: %w", err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Key not found in YAML file", "filePath", y.Path, "key", key)
		}
		return nil
	}

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Found key in YAML file", "filePath", y.Path, "key", key, "valueType", reflect.TypeOf(value))
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := yaml.Marshal(value)
	if err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to marshal value", "filePath", y.Path, "key", key, "error", err)
		}
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err = yaml.Unmarshal(valueBytes, target); err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to unmarshal value to target", "filePath", y.Path, "key", key, "error", err)
		}
		return fmt.Errorf("failed to unmarshal value to target: %w", err)
	}

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: FeedKey completed successfully", "filePath", y.Path, "key", key)
	}
	return nil
}
