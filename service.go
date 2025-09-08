package modular

import (
	"context"
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

// ServiceRegistryOption represents an option that can be applied to a service registry.
type ServiceRegistryOption func(*ScopedServiceRegistry) error

// ScopedServiceRegistry provides scoped service registry functionality.
// This extends the basic ServiceRegistry with scope-based instance management.
type ScopedServiceRegistry struct {
	*EnhancedServiceRegistry

	// serviceScopes maps service names to their configured scopes
	serviceScopes map[string]ServiceScope

	// scopeConfigs maps service names to their detailed scope configurations
	scopeConfigs map[string]ServiceScopeConfig

	// singletonInstances caches singleton service instances
	singletonInstances map[string]any

	// scopedInstances caches scoped service instances by scope key
	scopedInstances map[string]map[string]any // scope-key -> service-name -> instance
}

// NewServiceRegistry creates a new service registry with scope support.
// This is the constructor expected by the service registry tests.
func NewServiceRegistry() *ScopedServiceRegistry {
	return &ScopedServiceRegistry{
		EnhancedServiceRegistry: NewEnhancedServiceRegistry(),
		serviceScopes:           make(map[string]ServiceScope),
		scopeConfigs:            make(map[string]ServiceScopeConfig),
		singletonInstances:      make(map[string]any),
		scopedInstances:         make(map[string]map[string]any),
	}
}

// ApplyOption applies a service registry option to configure service scoping behavior.
func (r *ScopedServiceRegistry) ApplyOption(option ServiceRegistryOption) error {
	return option(r)
}

// GetServiceScope returns the configured scope for a service.
func (r *ScopedServiceRegistry) GetServiceScope(serviceName string) ServiceScope {
	if scope, exists := r.serviceScopes[serviceName]; exists {
		return scope
	}
	return GetDefaultServiceScope() // Return default scope if not configured
}

// Register registers a service factory with the scoped registry.
func (r *ScopedServiceRegistry) Register(name string, factory any) error {
	// For now, just delegate to the enhanced registry
	// In a full implementation, this would handle factory registration for scoped services
	_, err := r.EnhancedServiceRegistry.RegisterService(name, factory)
	return err
}

// Get retrieves a service instance respecting the configured scope.
func (r *ScopedServiceRegistry) Get(name string) (any, error) {
	scope := r.GetServiceScope(name)

	switch scope {
	case ServiceScopeSingleton:
		return r.getSingletonInstance(name)
	case ServiceScopeTransient:
		return r.getTransientInstance(name)
	default:
		return r.getDefaultInstance(name)
	}
}

// GetWithContext retrieves a service instance with context for scoped services.
func (r *ScopedServiceRegistry) GetWithContext(ctx context.Context, name string) (any, error) {
	scope := r.GetServiceScope(name)

	// Note: Service scope detection works correctly

	if scope == ServiceScopeScoped {
		return r.getScopedInstance(ctx, name)
	}

	// For non-scoped services, context doesn't matter
	return r.Get(name)
}

// getSingletonInstance retrieves or creates a singleton service instance.
func (r *ScopedServiceRegistry) getSingletonInstance(name string) (any, error) {
	// Check if already instantiated
	if instance, exists := r.singletonInstances[name]; exists {
		return instance, nil
	}

	// Get the factory from the registry
	factory, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", name)
	}

	// Create instance using factory
	instance := r.createInstanceFromFactory(factory.Service)
	r.singletonInstances[name] = instance

	return instance, nil
}

// getTransientInstance creates a new transient service instance.
func (r *ScopedServiceRegistry) getTransientInstance(name string) (any, error) {
	// Get the factory from the registry
	factory, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", name)
	}

	// Always create a new instance for transient services
	return r.createInstanceFromFactory(factory.Service), nil
}

// getScopedInstance retrieves or creates a scoped service instance.
func (r *ScopedServiceRegistry) getScopedInstance(ctx context.Context, name string) (any, error) {
	// Extract scope key from context
	config := r.scopeConfigs[name]
	scopeKey := r.extractScopeKey(ctx, config.ScopeKey)

	// Check if instance exists in scope
	if scopeInstances, exists := r.scopedInstances[scopeKey]; exists {
		if instance, exists := scopeInstances[name]; exists {
			return instance, nil
		}
	}

	// Create new instance for this scope
	factory, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", name)
	}

	instance := r.createInstanceFromFactory(factory.Service)

	// Store in scope cache
	if r.scopedInstances[scopeKey] == nil {
		r.scopedInstances[scopeKey] = make(map[string]any)
	}
	r.scopedInstances[scopeKey][name] = instance

	return instance, nil
}

// getDefaultInstance retrieves service using default registry behavior.
func (r *ScopedServiceRegistry) getDefaultInstance(name string) (any, error) {
	entry, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", name)
	}

	return r.createInstanceFromFactory(entry.Service), nil
}

// createInstanceFromFactory creates an instance from a factory function or returns the service directly.
func (r *ScopedServiceRegistry) createInstanceFromFactory(factory any) any {
	// Check if it's a factory function
	factoryValue := reflect.ValueOf(factory)
	if factoryValue.Kind() == reflect.Func {
		// Call the factory function
		results := factoryValue.Call(nil)
		if len(results) > 0 {
			return results[0].Interface()
		}
	}

	// Return the service directly if not a factory
	return factory
}

// extractScopeKey extracts the scope key value from context.
func (r *ScopedServiceRegistry) extractScopeKey(ctx context.Context, scopeKeyName string) string {
	// Use the same key type as WithScopeContext
	key := scopeContextKeyType(scopeKeyName)

	if value := ctx.Value(key); value != nil {
		if strValue, ok := value.(string); ok {
			return strValue
		}
	}

	return "default-scope"
}

// WithServiceScope creates a service registry option to configure service scope.
func WithServiceScope(serviceName string, scope ServiceScope) ServiceRegistryOption {
	return WithServiceScopeConfig(serviceName, GetDefaultScopeConfig(scope))
}

// WithServiceScopeConfig creates a service registry option with detailed scope configuration.
func WithServiceScopeConfig(serviceName string, config ServiceScopeConfig) ServiceRegistryOption {
	return func(registry *ScopedServiceRegistry) error {
		registry.serviceScopes[serviceName] = config.Scope
		registry.scopeConfigs[serviceName] = config
		return nil
	}
}
