package feeders

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/golobby/config/v3/pkg/feeder"
)

// Feeder interface for common operations
type Feeder interface {
	Feed(target interface{}) error
}

// feedKey is a common helper function for extracting specific keys from config files
func feedKey(
	feeder Feeder,
	key string,
	target interface{},
	marshalFunc func(interface{}) ([]byte, error),
	unmarshalFunc func([]byte, interface{}) error,
	fileType string,
) error {
	// Create a temporary map to hold all data
	var allData map[string]interface{}

	// Use the feeder to read the file
	if err := feeder.Feed(&allData); err != nil {
		return fmt.Errorf("failed to read %s: %w", fileType, err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		return nil
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := marshalFunc(value)
	if err != nil {
		return fmt.Errorf("failed to marshal %s data: %w", fileType, err)
	}

	if err = unmarshalFunc(valueBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal %s data: %w", fileType, err)
	}

	return nil
}

// JSONFeeder is a feeder that reads JSON files with optional verbose debug logging
type JSONFeeder struct {
	feeder.Json
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
}

// NewJSONFeeder creates a new JSONFeeder that reads from the specified JSON file
func NewJSONFeeder(filePath string) JSONFeeder {
	return JSONFeeder{
		Json:         feeder.Json{Path: filePath},
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (j *JSONFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	j.verboseDebug = enabled
	j.logger = logger
	if enabled && logger != nil {
		j.logger.Debug("Verbose JSON feeder debugging enabled")
	}
}

// Feed reads the JSON file and populates the provided structure
func (j JSONFeeder) Feed(structure interface{}) error {
	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Starting feed process", "filePath", j.Path, "structureType", reflect.TypeOf(structure))
	}

	err := j.Json.Feed(structure)
	if j.verboseDebug && j.logger != nil {
		if err != nil {
			j.logger.Debug("JSONFeeder: Feed completed with error", "filePath", j.Path, "error", err)
		} else {
			j.logger.Debug("JSONFeeder: Feed completed successfully", "filePath", j.Path)
		}
	}
	if err != nil {
		return fmt.Errorf("json feed error: %w", err)
	}
	return nil
}

// FeedKey reads a JSON file and extracts a specific key
func (j JSONFeeder) FeedKey(key string, target interface{}) error {
	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Starting FeedKey process", "filePath", j.Path, "key", key, "targetType", reflect.TypeOf(target))
	}

	err := feedKey(j, key, target, json.Marshal, json.Unmarshal, "JSON file")

	if j.verboseDebug && j.logger != nil {
		if err != nil {
			j.logger.Debug("JSONFeeder: FeedKey completed with error", "filePath", j.Path, "key", key, "error", err)
		} else {
			j.logger.Debug("JSONFeeder: FeedKey completed successfully", "filePath", j.Path, "key", key)
		}
	}
	return err
}
