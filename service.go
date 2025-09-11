package modular

import (
	"fmt"
	"reflect"
)

// ServiceRegistry allows registration and retrieval of services by name.
// Services are stored as interface{} values and must be type-asserted
// when retrieved. The registry supports both concrete types and interfaces.
//
// Services enable loose coupling between modules by providing a shared
// registry where modules can publish functionality for others to consume.
type ServiceRegistry map[string]any

// ServiceRegistryEntry represents an enhanced service registry entry
// that tracks both the service instance and its providing module.
type ServiceRegistryEntry struct {
	// Service is the actual service instance
	Service any

	// ModuleName is the name of the module that provided this service
	ModuleName string

	// ModuleType is the reflect.Type of the module that provided this service
	ModuleType reflect.Type

	// OriginalName is the original name requested when registering the service
	OriginalName string

	// ActualName is the final name used in the registry (may be modified for uniqueness)
	ActualName string
}

// EnhancedServiceRegistry provides enhanced service registry functionality
// that tracks module associations and handles automatic conflict resolution.
type EnhancedServiceRegistry struct {
	// services maps service names to their registry entries
	services map[string]*ServiceRegistryEntry

	// moduleServices maps module names to their provided services
	moduleServices map[string][]string

	// nameCounters tracks usage counts for conflict resolution
	nameCounters map[string]int

	// currentModule tracks the module currently being initialized
	currentModule Module
}

// NewEnhancedServiceRegistry creates a new enhanced service registry.
func NewEnhancedServiceRegistry() *EnhancedServiceRegistry {
	return &EnhancedServiceRegistry{
		services:       make(map[string]*ServiceRegistryEntry),
		moduleServices: make(map[string][]string),
		nameCounters:   make(map[string]int),
	}
}

// SetCurrentModule sets the module that is currently being initialized.
// This is used to track which module is registering services.
func (r *EnhancedServiceRegistry) SetCurrentModule(module Module) {
	r.currentModule = module
}

// ClearCurrentModule clears the current module context.
func (r *EnhancedServiceRegistry) ClearCurrentModule() {
	r.currentModule = nil
}

// RegisterService registers a service with automatic conflict resolution.
// If a service name conflicts, it will automatically append module information.
func (r *EnhancedServiceRegistry) RegisterService(name string, service any) (string, error) {
	var moduleName string
	var moduleType reflect.Type

	if r.currentModule != nil {
		moduleName = r.currentModule.Name()
		moduleType = reflect.TypeOf(r.currentModule)
	}

	// Generate unique name handling conflicts
	actualName := r.generateUniqueName(name, moduleName, moduleType)

	// Create registry entry
	entry := &ServiceRegistryEntry{
		Service:      service,
		ModuleName:   moduleName,
		ModuleType:   moduleType,
		OriginalName: name,
		ActualName:   actualName,
	}

	// Register the service
	r.services[actualName] = entry

	// Track module associations
	if moduleName != "" {
		r.moduleServices[moduleName] = append(r.moduleServices[moduleName], actualName)
	}

	return actualName, nil
}

// GetService retrieves a service by name.
func (r *EnhancedServiceRegistry) GetService(name string) (any, bool) {
	entry, exists := r.services[name]
	if !exists {
		return nil, false
	}
	return entry.Service, true
}

// GetServiceEntry retrieves the full service registry entry.
func (r *EnhancedServiceRegistry) GetServiceEntry(name string) (*ServiceRegistryEntry, bool) {
	entry, exists := r.services[name]
	return entry, exists
}

// GetServicesByModule returns all services provided by a specific module.
func (r *EnhancedServiceRegistry) GetServicesByModule(moduleName string) []string {
	return r.moduleServices[moduleName]
}

// GetServicesByInterface returns all services that implement the given interface.
func (r *EnhancedServiceRegistry) GetServicesByInterface(interfaceType reflect.Type) []*ServiceRegistryEntry {
	var results []*ServiceRegistryEntry

	for _, entry := range r.services {
		if entry.Service == nil {
			continue // Skip nil services
		}
		serviceType := reflect.TypeOf(entry.Service)
		if serviceType != nil && serviceType.Implements(interfaceType) {
			results = append(results, entry)
		}
	}

	return results
}

// AsServiceRegistry returns a backwards-compatible ServiceRegistry view.
func (r *EnhancedServiceRegistry) AsServiceRegistry() ServiceRegistry {
	registry := make(ServiceRegistry)
	for name, entry := range r.services {
		registry[name] = entry.Service
	}
	return registry
}

// generateUniqueName creates a unique service name handling conflicts.
func (r *EnhancedServiceRegistry) generateUniqueName(originalName, moduleName string, moduleType reflect.Type) string {
	// Try original name first
	if r.nameCounters[originalName] == 0 {
		r.nameCounters[originalName] = 1
		return originalName
	}

	// Name conflict exists - try with module name
	if moduleName != "" {
		moduleBasedName := fmt.Sprintf("%s.%s", originalName, moduleName)
		if r.nameCounters[moduleBasedName] == 0 {
			r.nameCounters[moduleBasedName] = 1
			return moduleBasedName
		}
	}

	// Still conflicts - try with module type name
	if moduleType != nil {
		typeName := moduleType.Elem().Name()
		if typeName == "" {
			typeName = moduleType.String()
		}
		typeBasedName := fmt.Sprintf("%s.%s", originalName, typeName)
		if r.nameCounters[typeBasedName] == 0 {
			r.nameCounters[typeBasedName] = 1
			return typeBasedName
		}
	}

	// Final fallback - append counter
	counter := r.nameCounters[originalName] + 1
	r.nameCounters[originalName] = counter
	return fmt.Sprintf("%s.%d", originalName, counter)
}

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
