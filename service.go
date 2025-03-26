package modular

import "reflect"

// ServiceRegistry allows registration and retrieval of services
type ServiceRegistry map[string]any

// Service defines a service with metadata
type Service struct {
	Name        string
	Description string
	Instance    any
}

// ServiceDependency defines a dependency on a service
type ServiceDependency struct {
	Name               string
	Required           bool
	Type               reflect.Type // Concrete type (if known)
	SatisfiesInterface reflect.Type // Interface type (if known)
}
