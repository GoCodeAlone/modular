package feeders

import (
	"encoding/json"
	"fmt"

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

// JSONFeeder is a feeder that reads JSON files
type JSONFeeder struct {
	feeder.Json
}

// NewJSONFeeder creates a new JSONFeeder that reads from the specified JSON file
func NewJSONFeeder(filePath string) JSONFeeder {
	return JSONFeeder{feeder.Json{Path: filePath}}
}

// FeedKey reads a JSON file and extracts a specific key
func (j JSONFeeder) FeedKey(key string, target interface{}) error {
	return feedKey(j, key, target, json.Marshal, json.Unmarshal, "JSON file")
}
