package feeders

import (
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/golobby/config/v3/pkg/feeder"
)

// TomlFeeder is a feeder that reads TOML files with optional verbose debug logging
type TomlFeeder struct {
	feeder.Toml
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
}

// NewTomlFeeder creates a new TomlFeeder that reads from the specified TOML file
func NewTomlFeeder(filePath string) TomlFeeder {
	return TomlFeeder{
		Toml:         feeder.Toml{Path: filePath},
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (t *TomlFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	t.verboseDebug = enabled
	t.logger = logger
	if enabled && logger != nil {
		t.logger.Debug("Verbose TOML feeder debugging enabled")
	}
}

// Feed reads the TOML file and populates the provided structure
func (t TomlFeeder) Feed(structure interface{}) error {
	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Starting feed process", "filePath", t.Path, "structureType", reflect.TypeOf(structure))
	}

	err := t.Toml.Feed(structure)
	if t.verboseDebug && t.logger != nil {
		if err != nil {
			t.logger.Debug("TomlFeeder: Feed completed with error", "filePath", t.Path, "error", err)
		} else {
			t.logger.Debug("TomlFeeder: Feed completed successfully", "filePath", t.Path)
		}
	}
	return err
}

// FeedKey reads a TOML file and extracts a specific key
func (t TomlFeeder) FeedKey(key string, target interface{}) error {
	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Starting FeedKey process", "filePath", t.Path, "key", key, "targetType", reflect.TypeOf(target))
	}

	err := feedKey(t, key, target, toml.Marshal, toml.Unmarshal, "TOML file")

	if t.verboseDebug && t.logger != nil {
		if err != nil {
			t.logger.Debug("TomlFeeder: FeedKey completed with error", "filePath", t.Path, "key", key, "error", err)
		} else {
			t.logger.Debug("TomlFeeder: FeedKey completed successfully", "filePath", t.Path, "key", key)
		}
	}
	return err
}
