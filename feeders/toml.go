package feeders

import (
	"github.com/BurntSushi/toml"
	"github.com/golobby/config/v3/pkg/feeder"
)

// TomlFeeder is a feeder that reads TOML files
type TomlFeeder struct {
	feeder.Toml
}

// NewTomlFeeder creates a new TomlFeeder that reads from the specified TOML file
func NewTomlFeeder(filePath string) TomlFeeder {
	return TomlFeeder{feeder.Toml{Path: filePath}}
}

// FeedKey reads a TOML file and extracts a specific key
func (t TomlFeeder) FeedKey(key string, target interface{}) error {
	return feedKey(t, key, target, toml.Marshal, toml.Unmarshal, "TOML file")
}
