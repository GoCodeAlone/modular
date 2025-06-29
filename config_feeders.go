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

// InstanceAwareFeeder provides functionality for feeding multiple instances of the same configuration type
type InstanceAwareFeeder interface {
	ComplexFeeder
	// FeedInstances feeds multiple instances from a map[string]ConfigType
	FeedInstances(instances interface{}) error
}

// InstancePrefixFunc is a function that generates a prefix for an instance key
type InstancePrefixFunc = feeders.InstancePrefixFunc

// NewInstanceAwareEnvFeeder creates a new instance-aware environment variable feeder
func NewInstanceAwareEnvFeeder(prefixFunc InstancePrefixFunc) InstanceAwareFeeder {
	return feeders.NewInstanceAwareEnvFeeder(prefixFunc)
}
