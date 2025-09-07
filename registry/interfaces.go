// Package registry defines interfaces for service registration and discovery
package registry

import (
	"context"
	"reflect"
	"time"
)

// ServiceRegistry defines the interface for service registration and resolution
type ServiceRegistry interface {
	// Register registers a service with the registry
	Register(ctx context.Context, registration *ServiceRegistration) error

	// Unregister removes a service from the registry
	Unregister(ctx context.Context, name string) error

	// ResolveByName resolves a service by its registered name
	ResolveByName(ctx context.Context, name string) (interface{}, error)

	// ResolveByInterface resolves a service by its interface type
	ResolveByInterface(ctx context.Context, interfaceType reflect.Type) (interface{}, error)

	// ResolveAllByInterface resolves all services implementing an interface
	ResolveAllByInterface(ctx context.Context, interfaceType reflect.Type) ([]interface{}, error)

	// List returns all registered services
	List(ctx context.Context) ([]*ServiceEntry, error)

	// ListByScope returns services in a specific scope
	ListByScope(ctx context.Context, scope ServiceScope) ([]*ServiceEntry, error)

	// Exists checks if a service with the given name exists
	Exists(ctx context.Context, name string) (bool, error)

	// GetDependencies returns the dependency graph for services
	GetDependencies(ctx context.Context) (*DependencyGraph, error)
}

// ServiceResolver defines advanced service resolution capabilities
type ServiceResolver interface {
	// ResolveWithTags resolves services matching specific tags
	ResolveWithTags(ctx context.Context, tags []string) ([]interface{}, error)

	// ResolveWithFilter resolves services matching a custom filter
	ResolveWithFilter(ctx context.Context, filter ServiceFilter) ([]interface{}, error)

	// ResolveLazy returns a lazy resolver for deferred service resolution
	ResolveLazy(ctx context.Context, name string) LazyResolver

	// ResolveOptional resolves a service if available, returns nil if not found
	ResolveOptional(ctx context.Context, name string) (interface{}, error)
}

// ServiceValidator defines validation capabilities for service registrations
type ServiceValidator interface {
	// ValidateRegistration validates a service registration before allowing it
	ValidateRegistration(ctx context.Context, registration *ServiceRegistration) error

	// ValidateConflict checks for registration conflicts and suggests resolutions
	ValidateConflict(ctx context.Context, registration *ServiceRegistration) (*ConflictAnalysis, error)

	// ValidateDependencies checks if service dependencies can be satisfied
	ValidateDependencies(ctx context.Context, dependencies []string) error
}

// ServiceRegistration represents a service registration request
type ServiceRegistration struct {
	Name           string                 `json:"name"`
	Service        interface{}            `json:"-"` // The actual service instance
	InterfaceTypes []reflect.Type         `json:"-"` // Interface types this service implements
	Priority       int                    `json:"priority"`
	Scope          ServiceScope           `json:"scope"`
	Tags           []string               `json:"tags,omitempty"`
	Dependencies   []string               `json:"dependencies,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	HealthChecker  HealthChecker          `json:"-"` // Optional health checker for the service

	// Lifecycle hooks
	OnStart func(ctx context.Context) error `json:"-"`
	OnStop  func(ctx context.Context) error `json:"-"`

	// Registration metadata
	RegisteredBy string    `json:"registered_by"` // Module or component that registered this service
	RegisteredAt time.Time `json:"registered_at"`
	Version      string    `json:"version,omitempty"`
}

// ServiceEntry represents a registered service in the registry
type ServiceEntry struct {
	Registration    *ServiceRegistration `json:"registration"`
	Status          ServiceStatus        `json:"status"`
	LastHealthCheck *time.Time           `json:"last_health_check,omitempty"`
	HealthStatus    HealthStatus         `json:"health_status"`
	Usage           *UsageStatistics     `json:"usage,omitempty"`

	// Conflict resolution
	ActualName      string   `json:"actual_name"`                // The name after conflict resolution
	ConflictedNames []string `json:"conflicted_names,omitempty"` // Names that conflicted

	// Runtime information
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	AccessedAt time.Time `json:"accessed_at"`
}

// DependencyGraph represents the service dependency relationships
type DependencyGraph struct {
	Nodes map[string]*DependencyNode `json:"nodes"`
	Edges []*DependencyEdge          `json:"edges"`
}

// DependencyNode represents a service in the dependency graph
type DependencyNode struct {
	ServiceName  string            `json:"service_name"`
	Status       ServiceStatus     `json:"status"`
	Dependencies []string          `json:"dependencies"`
	Dependents   []string          `json:"dependents"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// DependencyEdge represents a dependency relationship
type DependencyEdge struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	Type     DependencyType    `json:"type"`
	Required bool              `json:"required"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ConflictAnalysis provides information about service registration conflicts
type ConflictAnalysis struct {
	HasConflict      bool                    `json:"has_conflict"`
	ConflictingEntry *ServiceEntry           `json:"conflicting_entry,omitempty"`
	Resolution       ConflictResolution      `json:"resolution"`
	Suggestions      []*ResolutionSuggestion `json:"suggestions,omitempty"`
	ResolvedName     string                  `json:"resolved_name,omitempty"`
}

// ResolutionSuggestion suggests ways to resolve registration conflicts
type ResolutionSuggestion struct {
	Type        SuggestionType `json:"type"`
	Description string         `json:"description"`
	NewName     string         `json:"new_name,omitempty"`
	Action      string         `json:"action"`
}

// UsageStatistics tracks how often a service is accessed
type UsageStatistics struct {
	AccessCount         int64         `json:"access_count"`
	LastAccessTime      time.Time     `json:"last_access_time"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	ErrorCount          int64         `json:"error_count"`
	LastErrorTime       *time.Time    `json:"last_error_time,omitempty"`
}

// LazyResolver provides deferred service resolution
type LazyResolver interface {
	// Resolve resolves the service when actually needed
	Resolve(ctx context.Context) (interface{}, error)

	// IsResolved returns true if the service has been resolved
	IsResolved() bool

	// ServiceName returns the name of the service being resolved
	ServiceName() string
}

// ServiceFilter defines a filter function for service resolution
type ServiceFilter func(entry *ServiceEntry) bool

// HealthChecker defines health checking for services
type HealthChecker interface {
	// CheckHealth checks the health of the service
	CheckHealth(ctx context.Context, service interface{}) error

	// Name returns the name of this health checker
	Name() string
}

// ServiceScope defines the scope of service availability
type ServiceScope string

const (
	ScopeGlobal   ServiceScope = "global"   // Available globally
	ScopeTenant   ServiceScope = "tenant"   // Scoped to specific tenant
	ScopeInstance ServiceScope = "instance" // Scoped to specific instance
	ScopeModule   ServiceScope = "module"   // Scoped to specific module
)

// ServiceStatus represents the current status of a service
type ServiceStatus string

const (
	ServiceStatusActive   ServiceStatus = "active"
	ServiceStatusInactive ServiceStatus = "inactive"
	ServiceStatusStarting ServiceStatus = "starting"
	ServiceStatusStopping ServiceStatus = "stopping"
	ServiceStatusError    ServiceStatus = "error"
)

// HealthStatus represents the health status of a service
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// DependencyType represents the type of dependency relationship
type DependencyType string

const (
	DependencyTypeRequired DependencyType = "required"
	DependencyTypeOptional DependencyType = "optional"
	DependencyTypeWeak     DependencyType = "weak"
)

// ConflictResolution defines how service name conflicts are resolved
type ConflictResolution string

const (
	ConflictResolutionError     ConflictResolution = "error"     // Fail the registration
	ConflictResolutionOverwrite ConflictResolution = "overwrite" // Replace existing service
	ConflictResolutionRename    ConflictResolution = "rename"    // Auto-rename the new service
	ConflictResolutionPriority  ConflictResolution = "priority"  // Use priority to decide
	ConflictResolutionIgnore    ConflictResolution = "ignore"    // Ignore the new registration
)

// SuggestionType defines types of conflict resolution suggestions
type SuggestionType string

const (
	SuggestionTypeRename    SuggestionType = "rename"
	SuggestionTypeNamespace SuggestionType = "namespace"
	SuggestionTypeScope     SuggestionType = "scope"
	SuggestionTypePriority  SuggestionType = "priority"
)

// RegistryConfig represents configuration for the service registry
type RegistryConfig struct {
	ConflictResolution   ConflictResolution `json:"conflict_resolution"`
	EnableHealthChecking bool               `json:"enable_health_checking"`
	HealthCheckInterval  time.Duration      `json:"health_check_interval"`
	EnableUsageTracking  bool               `json:"enable_usage_tracking"`
	CleanupInterval      time.Duration      `json:"cleanup_interval"`
	MaxServiceAge        time.Duration      `json:"max_service_age"`
	EnableLazyResolution bool               `json:"enable_lazy_resolution"`
}
