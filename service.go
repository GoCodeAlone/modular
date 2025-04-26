package modular

import "reflect"

// ServiceRegistry allows registration and retrieval of services
type ServiceRegistry map[string]any

// ServiceProvider defines a service with metadata
type ServiceProvider struct {
	Name        string
	Description string
	Instance    any
}

// ServiceDependency defines a dependency on a service
type ServiceDependency struct {
	Name               string       // Service name to lookup (can be empty for interface-based lookup)
	Required           bool         // If true, application fails to start if service is missing
	Type               reflect.Type // Concrete type (if known)
	SatisfiesInterface reflect.Type // Interface type (if known)
	MatchByInterface   bool         // If true, find first service that satisfies interface type
}
