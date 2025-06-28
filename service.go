package modular

import "reflect"

// ServiceRegistry allows registration and retrieval of services by name.
// Services are stored as interface{} values and must be type-asserted
// when retrieved. The registry supports both concrete types and interfaces.
//
// Services enable loose coupling between modules by providing a shared
// registry where modules can publish functionality for others to consume.
type ServiceRegistry map[string]any

// ServiceProvider defines a service offered by a module.
// Services are registered in the application's service registry and can
// be consumed by other modules that declare them as dependencies.
//
// A service provider encapsulates:
//   - Name: unique identifier for service lookup
//   - Description: human-readable description for documentation
//   - Instance: the actual service implementation (interface{})
type ServiceProvider struct {
	// Name is the unique identifier for this service.
	// Other modules reference this service by this exact name.
	// Should be descriptive and follow naming conventions like "database", "logger", "cache".
	Name string

	// Description provides human-readable documentation for this service.
	// Used for debugging and documentation purposes.
	// Example: "PostgreSQL database connection pool"
	Description string

	// Instance is the actual service implementation.
	// Can be any type - struct, interface implementation, function, etc.
	// Consuming modules are responsible for type assertion.
	Instance any
}

// ServiceDependency defines a requirement for a service from another module.
// Dependencies can be matched either by exact name or by interface type.
// The framework handles dependency resolution and injection automatically.
//
// There are two main patterns for service dependencies:
//
//  1. Name-based lookup:
//     ServiceDependency{Name: "database", Required: true}
//
//  2. Interface-based lookup:
//     ServiceDependency{
//     Name: "logger",
//     MatchByInterface: true,
//     SatisfiesInterface: reflect.TypeOf((*Logger)(nil)).Elem(),
//     Required: true,
//     }
type ServiceDependency struct {
	// Name is the service identifier to lookup.
	// For interface-based matching, this is used as the key in the
	// injected services map but may not correspond to a registered service name.
	Name string

	// Required indicates whether the application should fail to start
	// if this service is not available. Optional services (Required: false)
	// will be silently ignored if not found.
	Required bool

	// Type specifies the concrete type expected for this service.
	// Used for additional type checking during dependency resolution.
	// Optional - if nil, no concrete type checking is performed.
	Type reflect.Type

	// SatisfiesInterface specifies an interface that the service must implement.
	// Used with MatchByInterface to find services by interface rather than name.
	// Obtain with: reflect.TypeOf((*InterfaceName)(nil)).Elem()
	SatisfiesInterface reflect.Type

	// MatchByInterface enables interface-based service lookup.
	// When true, the framework will search for any service that implements
	// SatisfiesInterface rather than looking up by exact name.
	// Useful for loose coupling where modules depend on interfaces rather than specific implementations.
	MatchByInterface bool
}
