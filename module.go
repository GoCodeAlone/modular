// Package modular provides a flexible, modular application framework for Go.
// It supports configuration management, dependency injection, service registration,
// and multi-tenant functionality.
package modular

import "context"

// Module represents a registrable component in the application
type Module interface {
	// Name returns the unique identifier for this module
	Name() string
	// Init Initialize the module with the application context
	Init(app Application) error
}

// Configurable is an interface for modules that can have configuration
type Configurable interface {
	// RegisterConfig registers configuration requirements
	RegisterConfig(app Application) error
}

// DependencyAware is an interface for modules that can have dependencies
type DependencyAware interface {
	// Dependencies returns names of other modules this module depends on
	Dependencies() []string
}

// ServiceAware is an interface for modules that can provide or require services
type ServiceAware interface {
	// ProvidesServices returns a list of services provided by this module
	ProvidesServices() []ServiceProvider
	// RequiresServices returns a list of services required by this module
	RequiresServices() []ServiceDependency
}

// Startable is an interface for modules that can be started
type Startable interface {
	Start(ctx context.Context) error
}

// Stoppable is an interface for modules that can be stopped
type Stoppable interface {
	Stop(ctx context.Context) error
}

// Constructable is an interface for modules that can be constructed with a constructor
type Constructable interface {
	// Constructor returns a function to construct this module
	Constructor() ModuleConstructor
}

// ModuleConstructor is a function type that creates module instances with dependency injection
type ModuleConstructor func(app Application, services map[string]any) (Module, error)

// ModuleWithConstructor defines modules that support constructor-based dependency injection
type ModuleWithConstructor interface {
	Module
	Constructable
}

// ModuleRegistry represents a svcRegistry of modules
type ModuleRegistry map[string]Module
