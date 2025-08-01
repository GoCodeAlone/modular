package modular

import (
	"github.com/GoCodeAlone/modular/feeders"
)

// Feeder defines the interface for configuration feeders that provide configuration data.
type Feeder interface {
	// Feed gets a struct and feeds it using configuration data.
	Feed(structure interface{}) error
}

// ConfigFeeders provides a default set of configuration feeders for common use cases
var ConfigFeeders = []Feeder{
	feeders.NewEnvFeeder(),
}

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

// VerboseAwareFeeder provides functionality for verbose debug logging during configuration feeding
type VerboseAwareFeeder interface {
	// SetVerboseDebug enables or disables verbose debug logging
	SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) })
}

// VerboseLogger provides a minimal logging interface to avoid circular dependencies
type VerboseLogger interface {
	Debug(msg string, args ...any)
}

// ModuleAwareFeeder provides functionality for feeders that can receive module context
// during configuration feeding. This allows feeders to customize behavior based on
// which module's configuration is being processed.
type ModuleAwareFeeder interface {
	Feeder
	// FeedWithModuleContext feeds configuration with module context information.
	// The moduleName parameter provides the name of the module whose configuration
	// is being processed, allowing the feeder to customize its behavior accordingly.
	FeedWithModuleContext(structure interface{}, moduleName string) error
}

// InstancePrefixFunc is a function that generates a prefix for an instance key
type InstancePrefixFunc = feeders.InstancePrefixFunc

// NewInstanceAwareEnvFeeder creates a new instance-aware environment variable feeder
func NewInstanceAwareEnvFeeder(prefixFunc InstancePrefixFunc) InstanceAwareFeeder {
	return feeders.NewInstanceAwareEnvFeeder(prefixFunc)
}
