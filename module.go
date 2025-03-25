package modular

// Module represents a registrable component in the application
type Module interface {
	// RegisterConfig registers configuration requirements
	RegisterConfig(app *Application)

	// Init Initialize the module with the application context
	Init(app *Application) error

	// Name returns the unique identifier for this module
	Name() string

	// Dependencies returns names of other modules this module depends on
	Dependencies() []string

	ProvidesServices() []Service
	RequiresServices() []ServiceDependency
}

type ModuleConstructor func(app *Application, services map[string]any) (Module, error)

type ModuleWithConstructor interface {
	Module
	Constructor() ModuleConstructor
}

// ModuleRegistry represents a svcRegistry of modules
type ModuleRegistry map[string]Module
