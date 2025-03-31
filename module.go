package modular

import "context"

// Module represents a registrable component in the application
type Module interface {
	// RegisterConfig registers configuration requirements
	RegisterConfig(app Application)

	// Init Initialize the module with the application context
	Init(app Application) error

	// Start starts the module (non-blocking)
	Start(ctx context.Context) error

	// Stop gracefully stops the module
	Stop(ctx context.Context) error

	// Name returns the unique identifier for this module
	Name() string

	// Dependencies returns names of other modules this module depends on
	Dependencies() []string

	// ProvidesServices returns a list of services provided by this module
	ProvidesServices() []ServiceProvider

	// RequiresServices returns a list of services required by this module
	RequiresServices() []ServiceDependency
}

type ModuleConstructor func(app *StdApplication, services map[string]any) (Module, error)

type ModuleWithConstructor interface {
	Module
	Constructor() ModuleConstructor
}

// ModuleRegistry represents a svcRegistry of modules
type ModuleRegistry map[string]Module
