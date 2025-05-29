package modular

import (
	"github.com/golobby/config/v3"

	"github.com/GoCodeAlone/modular/feeders"
)

// ConfigFeeders provides a default set of configuration feeders for common use cases
var ConfigFeeders = []Feeder{
	feeders.EnvFeeder{},
}

// Feeder aliases
type Feeder = config.Feeder

// ComplexFeeder extends the basic Feeder interface with additional functionality for complex configuration scenarios
type ComplexFeeder interface {
	Feeder
	FeedKey(string, interface{}) error
}
