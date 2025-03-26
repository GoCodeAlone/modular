package feeders

import (
	"encoding/json"
	"fmt"
	"github.com/golobby/config/v3/pkg/feeder"
)

// JsonFeeder is a feeder that reads JSON files
type JsonFeeder struct {
	feeder.Json
}

func NewJsonFeeder(filePath string) JsonFeeder {
	return JsonFeeder{feeder.Json{Path: filePath}}
}

// FeedKey reads a JSON file and extracts a specific key
func (j JsonFeeder) FeedKey(key string, target interface{}) error {
	// Create a temporary map to hold all json data
	var allData map[string]interface{}

	// Use the embedded Json feeder to read the file
	if err := j.Feed(&allData); err != nil {
		return err
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		//return fmt.Errorf("key '%s' not found in json data", key)
		return nil
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal json data: %w", err)
	}

	if err = json.Unmarshal(valueBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal json data: %w", err)
	}

	return nil
}
