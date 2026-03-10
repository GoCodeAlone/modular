package modular

// Plugin is the minimal interface for a plugin bundle that provides modules.
type Plugin interface {
	Name() string
	Modules() []Module
}

// PluginWithHooks extends Plugin with initialization hooks.
type PluginWithHooks interface {
	Plugin
	InitHooks() []func(Application) error
}

// PluginWithServices extends Plugin with service definitions.
type PluginWithServices interface {
	Plugin
	Services() []ServiceDefinition
}

// ServiceDefinition describes a service provided by a plugin.
type ServiceDefinition struct {
	Name    string
	Service any
}
