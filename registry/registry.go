// Package registry provides service registration and discovery capabilities
package registry

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"
)

// Static errors for registry package
var (
	ErrRegisterNotImplemented              = errors.New("register method not fully implemented")
	ErrUnregisterNotImplemented            = errors.New("unregister method not fully implemented")
	ErrResolveByNameNotImplemented         = errors.New("resolve by name method not fully implemented")
	ErrResolveByInterfaceNotImplemented    = errors.New("resolve by interface method not fully implemented")
	ErrResolveAllByInterfaceNotImplemented = errors.New("resolve all by interface method not fully implemented")
	ErrListByScopeNotImplemented           = errors.New("list by scope method not yet implemented")
	ErrGetDependenciesNotImplemented       = errors.New("get dependencies method not yet implemented")
	ErrResolveWithTagsNotImplemented       = errors.New("resolve with tags method not yet implemented")
	ErrResolveWithFilterNotImplemented     = errors.New("resolve with filter method not yet implemented")
	ErrValidateRegistrationNotImplemented  = errors.New("validate registration method not fully implemented")
	ErrValidateConflictNotImplemented      = errors.New("validate conflict method not yet implemented")
	ErrValidateDependenciesNotImplemented  = errors.New("validate dependencies method not yet implemented")
	ErrServiceNotFound                     = errors.New("service not found")
	ErrNoServicesFoundForInterface         = errors.New("no services found implementing interface")
	ErrAmbiguousInterfaceResolution        = errors.New("ambiguous interface resolution: multiple services implement interface")
)

// Registry implements the ServiceRegistry interface with basic map-based storage
type Registry struct {
	mu         sync.RWMutex
	services   map[string]*ServiceEntry
	byType     map[reflect.Type][]*ServiceEntry
	config     *RegistryConfig
	validators []ServiceValidator
}

// NewRegistry creates a new service registry
func NewRegistry(config *RegistryConfig) *Registry {
	if config == nil {
		config = &RegistryConfig{
			ConflictResolution:   ConflictResolutionError,
			EnableHealthChecking: false,
			EnableUsageTracking:  false,
			EnableLazyResolution: false,
		}
	}

	return &Registry{
		services:   make(map[string]*ServiceEntry),
		byType:     make(map[reflect.Type][]*ServiceEntry),
		config:     config,
		validators: make([]ServiceValidator, 0),
	}
}

// Register registers a service with the registry
func (r *Registry) Register(ctx context.Context, registration *ServiceRegistration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	
	// Fill in registration metadata if not provided
	if registration.RegisteredAt.IsZero() {
		registration.RegisteredAt = now
	}

	// Check for existing service with the same name
	if existing, exists := r.services[registration.Name]; exists {
		// Handle conflict according to configuration
		resolved, err := r.resolveConflict(existing, registration)
		if err != nil {
			return err
		}
		if resolved.ActualName != registration.Name {
			// Service was renamed during conflict resolution
			registration.Name = resolved.ActualName
		}
	}

	entry := &ServiceEntry{
		Registration: registration,
		Status:       ServiceStatusActive,
		HealthStatus: HealthStatusUnknown,
		ActualName:   registration.Name,
		CreatedAt:    now,
		UpdatedAt:    now,
		AccessedAt:   now,
	}

	// Initialize usage statistics if tracking is enabled
	if r.config.EnableUsageTracking {
		entry.Usage = &UsageStatistics{
			AccessCount:    0,
			LastAccessTime: now,
		}
	}

	r.services[registration.Name] = entry

	// Index by interface types for O(1) lookup
	for _, interfaceType := range registration.InterfaceTypes {
		r.byType[interfaceType] = append(r.byType[interfaceType], entry)
	}

	return nil
}

// Unregister removes a service from the registry
func (r *Registry) Unregister(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.services[name]
	if !exists {
		return ErrServiceNotFound
	}

	// Remove from name index
	delete(r.services, name)

	// Remove from interface type indexes
	for _, interfaceType := range entry.Registration.InterfaceTypes {
		entries := r.byType[interfaceType]
		for i, e := range entries {
			if e == entry {
				// Remove this entry from the slice
				r.byType[interfaceType] = append(entries[:i], entries[i+1:]...)
				break
			}
		}
		// Clean up empty slices
		if len(r.byType[interfaceType]) == 0 {
			delete(r.byType, interfaceType)
		}
	}

	return nil
}

// ResolveByName resolves a service by its registered name
func (r *Registry) ResolveByName(ctx context.Context, name string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.services[name]
	if !exists {
		return nil, ErrServiceNotFound
	}

	// Update access time if usage tracking is enabled
	if r.config.EnableUsageTracking && entry.Usage != nil {
		entry.Usage.AccessCount++
		entry.Usage.LastAccessTime = time.Now()
		entry.AccessedAt = time.Now()
	}

	return entry.Registration.Service, nil
}

// ResolveByInterface resolves a service by its interface type
func (r *Registry) ResolveByInterface(ctx context.Context, interfaceType reflect.Type) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, exists := r.byType[interfaceType]
	if !exists || len(entries) == 0 {
		return nil, ErrNoServicesFoundForInterface
	}

	if len(entries) == 1 {
		// Single service, no ambiguity
		entry := entries[0]
		if r.config.EnableUsageTracking && entry.Usage != nil {
			entry.Usage.AccessCount++
			entry.Usage.LastAccessTime = time.Now()
			entry.AccessedAt = time.Now()
		}
		return entry.Registration.Service, nil
	}

	// Multiple services - need tie-breaking
	resolved, err := r.resolveTieBreak(entries)
	if err != nil {
		return nil, err
	}

	if r.config.EnableUsageTracking && resolved.Usage != nil {
		resolved.Usage.AccessCount++
		resolved.Usage.LastAccessTime = time.Now()
		resolved.AccessedAt = time.Now()
	}

	return resolved.Registration.Service, nil
}

// ResolveAllByInterface resolves all services implementing an interface
func (r *Registry) ResolveAllByInterface(ctx context.Context, interfaceType reflect.Type) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, exists := r.byType[interfaceType]
	if !exists {
		return nil, nil
	}

	services := make([]interface{}, len(entries))
	for i, entry := range entries {
		services[i] = entry.Registration.Service
		
		// Update usage statistics if enabled
		if r.config.EnableUsageTracking && entry.Usage != nil {
			entry.Usage.AccessCount++
			entry.Usage.LastAccessTime = time.Now()
			entry.AccessedAt = time.Now()
		}
	}

	return services, nil
}

// List returns all registered services
func (r *Registry) List(ctx context.Context) ([]*ServiceEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*ServiceEntry, 0, len(r.services))
	for _, entry := range r.services {
		entries = append(entries, entry)
	}

	return entries, nil
}

// ListByScope returns services in a specific scope
func (r *Registry) ListByScope(ctx context.Context, scope ServiceScope) ([]*ServiceEntry, error) {
	// TODO: Implement scope-based service listing
	return nil, ErrListByScopeNotImplemented
}

// Exists checks if a service with the given name exists
func (r *Registry) Exists(ctx context.Context, name string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.services[name]
	return exists, nil
}

// GetDependencies returns the dependency graph for services
func (r *Registry) GetDependencies(ctx context.Context) (*DependencyGraph, error) {
	// TODO: Implement dependency graph construction
	return nil, ErrGetDependenciesNotImplemented
}

// Resolver implements basic ServiceResolver interface
type Resolver struct {
	registry *Registry
}

// NewResolver creates a new service resolver
func NewResolver(registry *Registry) *Resolver {
	return &Resolver{registry: registry}
}

// ResolveWithTags resolves services matching specific tags
func (r *Resolver) ResolveWithTags(ctx context.Context, tags []string) ([]interface{}, error) {
	// TODO: Implement tag-based service resolution
	return nil, ErrResolveWithTagsNotImplemented
}

// ResolveWithFilter resolves services matching a custom filter
func (r *Resolver) ResolveWithFilter(ctx context.Context, filter ServiceFilter) ([]interface{}, error) {
	// TODO: Implement filter-based service resolution
	return nil, ErrResolveWithFilterNotImplemented
}

// ResolveLazy returns a lazy resolver for deferred service resolution
func (r *Resolver) ResolveLazy(ctx context.Context, name string) LazyResolver {
	// TODO: Implement lazy service resolution
	return &lazyResolver{
		registry:    r.registry,
		serviceName: name,
		resolved:    false,
		service:     nil,
	}
}

// ResolveOptional resolves a service if available, returns nil if not found
func (r *Resolver) ResolveOptional(ctx context.Context, name string) (interface{}, error) {
	service, err := r.registry.ResolveByName(ctx, name)
	if err != nil {
		// For optional resolution, we return nil service without error when not found
		if errors.Is(err, ErrServiceNotFound) || errors.Is(err, ErrResolveByNameNotImplemented) {
			return nil, nil
		}
		// Return other errors as-is
		return nil, err
	}
	return service, nil
}

// lazyResolver implements LazyResolver interface
type lazyResolver struct {
	registry    *Registry
	serviceName string
	resolved    bool
	service     interface{}
	mu          sync.Mutex
}

// Resolve resolves the service when actually needed
func (lr *lazyResolver) Resolve(ctx context.Context) (interface{}, error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.resolved {
		return lr.service, nil
	}

	service, err := lr.registry.ResolveByName(ctx, lr.serviceName)
	if err != nil {
		return nil, err
	}

	lr.service = service
	lr.resolved = true
	return service, nil
}

// IsResolved returns true if the service has been resolved
func (lr *lazyResolver) IsResolved() bool {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	return lr.resolved
}

// ServiceName returns the name of the service being resolved
func (lr *lazyResolver) ServiceName() string {
	return lr.serviceName
}

// Validator implements basic ServiceValidator interface
type Validator struct {
	rules []func(*ServiceRegistration) error
}

// NewValidator creates a new service validator
func NewValidator() *Validator {
	return &Validator{
		rules: make([]func(*ServiceRegistration) error, 0),
	}
}

// ValidateRegistration validates a service registration before allowing it
func (v *Validator) ValidateRegistration(ctx context.Context, registration *ServiceRegistration) error {
	// TODO: Implement registration validation
	for _, rule := range v.rules {
		if err := rule(registration); err != nil {
			return err
		}
	}
	return ErrValidateRegistrationNotImplemented
}

// ValidateConflict checks for registration conflicts and suggests resolutions
func (v *Validator) ValidateConflict(ctx context.Context, registration *ServiceRegistration) (*ConflictAnalysis, error) {
	// TODO: Implement conflict analysis
	return nil, ErrValidateConflictNotImplemented
}

// ValidateDependencies checks if service dependencies can be satisfied
func (v *Validator) ValidateDependencies(ctx context.Context, dependencies []string) error {
	// TODO: Implement dependency validation
	return ErrValidateDependenciesNotImplemented
}

// AddRule adds a validation rule
func (v *Validator) AddRule(rule func(*ServiceRegistration) error) {
	v.rules = append(v.rules, rule)
}

// resolveConflict handles service name conflicts according to the configured resolution strategy
func (r *Registry) resolveConflict(existing *ServiceEntry, new *ServiceRegistration) (*ServiceEntry, error) {
	now := time.Now()
	
	switch r.config.ConflictResolution {
	case ConflictResolutionError:
		return nil, errors.New("service registration conflict: service name already exists")
		
	case ConflictResolutionOverwrite:
		// Replace the existing service
		entry := &ServiceEntry{
			Registration: new,
			Status:       ServiceStatusActive,
			HealthStatus: HealthStatusUnknown,
			ActualName:   new.Name,
			CreatedAt:    now,
			UpdatedAt:    now,
			AccessedAt:   now,
		}
		if r.config.EnableUsageTracking {
			entry.Usage = &UsageStatistics{
				AccessCount:    0,
				LastAccessTime: now,
			}
		}
		return entry, nil
		
	case ConflictResolutionRename:
		// Auto-rename the new service
		resolvedName := r.findAvailableName(new.Name)
		new.Name = resolvedName
		entry := &ServiceEntry{
			Registration:    new,
			Status:          ServiceStatusActive,
			HealthStatus:    HealthStatusUnknown,
			ActualName:      resolvedName,
			ConflictedNames: []string{new.Name}, // Original name that conflicted
			CreatedAt:       now,
			UpdatedAt:       now,
			AccessedAt:      now,
		}
		if r.config.EnableUsageTracking {
			entry.Usage = &UsageStatistics{
				AccessCount:    0,
				LastAccessTime: now,
			}
		}
		return entry, nil
		
	case ConflictResolutionPriority:
		// Use priority to decide (higher priority wins)
		if new.Priority > existing.Registration.Priority {
			// New service has higher priority, replace existing
			entry := &ServiceEntry{
				Registration: new,
				Status:       ServiceStatusActive,
				HealthStatus: HealthStatusUnknown,
				ActualName:   new.Name,
				CreatedAt:    now,
				UpdatedAt:    now,
				AccessedAt:   now,
			}
			if r.config.EnableUsageTracking {
				entry.Usage = &UsageStatistics{
					AccessCount:    0,
					LastAccessTime: now,
				}
			}
			return entry, nil
		}
		// Existing service has higher or equal priority, ignore new registration
		return existing, nil
		
	case ConflictResolutionIgnore:
		// Keep existing service, ignore new registration
		return existing, nil
		
	default:
		return nil, errors.New("unknown conflict resolution strategy")
	}
}

// resolveTieBreak resolves ambiguity when multiple services implement the same interface
// Priority order: explicit name > priority > registration time (earliest wins)
func (r *Registry) resolveTieBreak(entries []*ServiceEntry) (*ServiceEntry, error) {
	if len(entries) == 0 {
		return nil, ErrNoServicesFoundForInterface
	}
	
	if len(entries) == 1 {
		return entries[0], nil
	}

	// Step 1: Check for explicit name matches (services with most specific names)
	// For now, we'll use the concept that shorter names are more explicit
	minNameLength := len(entries[0].ActualName)
	explicitEntries := []*ServiceEntry{entries[0]}
	
	for i := 1; i < len(entries); i++ {
		nameLen := len(entries[i].ActualName)
		if nameLen < minNameLength {
			minNameLength = nameLen
			explicitEntries = []*ServiceEntry{entries[i]}
		} else if nameLen == minNameLength {
			explicitEntries = append(explicitEntries, entries[i])
		}
	}
	
	if len(explicitEntries) == 1 {
		return explicitEntries[0], nil
	}

	// Step 2: Compare priorities (higher priority wins)
	maxPriority := explicitEntries[0].Registration.Priority
	priorityEntries := []*ServiceEntry{explicitEntries[0]}
	
	for i := 1; i < len(explicitEntries); i++ {
		priority := explicitEntries[i].Registration.Priority
		if priority > maxPriority {
			maxPriority = priority
			priorityEntries = []*ServiceEntry{explicitEntries[i]}
		} else if priority == maxPriority {
			priorityEntries = append(priorityEntries, explicitEntries[i])
		}
	}
	
	if len(priorityEntries) == 1 {
		return priorityEntries[0], nil
	}

	// Step 3: Use registration time (earliest wins)
	earliest := priorityEntries[0]
	for i := 1; i < len(priorityEntries); i++ {
		if priorityEntries[i].Registration.RegisteredAt.Before(earliest.Registration.RegisteredAt) {
			earliest = priorityEntries[i]
		}
	}

	// If we still have ties, format an error with all conflicting services
	if len(priorityEntries) > 1 {
		names := make([]string, 0, len(priorityEntries))
		for _, entry := range priorityEntries {
			names = append(names, entry.ActualName)
		}
		return nil, errors.New("ambiguous interface resolution: multiple services with equal priority and registration time: " + 
			"[" + joinStrings(names, ", ") + "]")
	}

	return earliest, nil
}

// findAvailableName finds an available name by appending a suffix
func (r *Registry) findAvailableName(baseName string) string {
	if _, exists := r.services[baseName]; !exists {
		return baseName
	}
	
	for i := 1; i < 1000; i++ { // Reasonable limit to prevent infinite loop
		candidate := baseName + "-" + intToString(i)
		if _, exists := r.services[candidate]; !exists {
			return candidate
		}
	}
	
	// Fallback to timestamp-based suffix
	return baseName + "-" + intToString(int(time.Now().Unix()%1000))
}

// intToString converts an integer to string (simple implementation)
func intToString(i int) string {
	if i == 0 {
		return "0"
	}
	
	negative := i < 0
	if negative {
		i = -i
	}
	
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0'+i%10)}, digits...)
		i /= 10
	}
	
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	
	return string(digits)
}

// joinStrings joins a slice of strings with a separator (utility function)
func joinStrings(strs []string, separator string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += separator + strs[i]
	}
	return result
}
